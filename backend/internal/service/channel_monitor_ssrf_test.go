package service

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

type monitorResolverFunc func(context.Context, string) ([]net.IPAddr, error)

func (f monitorResolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

func TestMonitorSSRFEndpointLiterals(t *testing.T) {
	// Literal addresses exercise URL and IP policy without DNS or HTTP traffic.
	for _, tc := range []struct {
		endpoint string
		want     error
	}{
		{"https://8.8.8.8", nil},
		{"https://[2606:4700:4700::1111]", nil},
		{"", ErrChannelMonitorInvalidEndpoint},
		{"http://8.8.8.8", ErrChannelMonitorEndpointScheme},
		{"https://8.8.8.8/v1", ErrChannelMonitorEndpointPath},
		{"https://8.8.8.8?x=1", ErrChannelMonitorEndpointPath},
		{"https://8.8.8.8#fragment", ErrChannelMonitorEndpointPath},
		{"https://localhost", ErrChannelMonitorEndpointPrivate},
		{"https://metadata.google.internal", ErrChannelMonitorEndpointPrivate},
		{"https://127.0.0.1", ErrChannelMonitorEndpointPrivate},
		{"https://169.254.169.254", ErrChannelMonitorEndpointPrivate},
		{"https://10.0.0.1", ErrChannelMonitorEndpointPrivate},
		{"https://100.64.0.1", ErrChannelMonitorEndpointPrivate},
		{"https://[::1]", ErrChannelMonitorEndpointPrivate},
		{"https://[::ffff:127.0.0.1]", ErrChannelMonitorEndpointPrivate},
		{"https://[fd00::1]", ErrChannelMonitorEndpointPrivate},
	} {
		t.Run(tc.endpoint, func(t *testing.T) {
			err := validateEndpoint(tc.endpoint)
			if tc.want == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.want)
			}
		})
	}
}

func TestMonitorSSRFResolverAndDialBoundaries(t *testing.T) {
	dnsErr := errors.New("synthetic DNS failure")
	for _, tc := range []struct {
		name    string
		ips     []string
		err     error
		blocked bool
	}{
		{"public v4 and v6", []string{"8.8.8.8", "2606:4700:4700::1111"}, nil, false},
		{"private", []string{"192.168.0.1"}, nil, true},
		{"public then private", []string{"8.8.8.8", "10.0.0.1"}, nil, true},
		{"private then public", []string{"10.0.0.1", "8.8.8.8"}, nil, true},
		{"public and mapped loopback", []string{"8.8.8.8", "::ffff:127.0.0.1"}, nil, true},
		{"empty answer", nil, nil, true},
		{"nil address", []string{"invalid"}, nil, true},
		{"DNS error", nil, dnsErr, false},
		{"cancelled lookup", nil, context.Canceled, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := monitorResolverFunc(func(ctx context.Context, host string) ([]net.IPAddr, error) {
				require.Equal(t, "synthetic.example", host)
				var addrs []net.IPAddr
				for _, ip := range tc.ips {
					addrs = append(addrs, net.IPAddr{IP: net.ParseIP(ip)})
				}
				return addrs, tc.err
			})
			blocked, err := isPrivateOrLoopbackHostWithResolver(context.Background(), "synthetic.example", resolver)
			require.Equal(t, tc.blocked, blocked)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
			dials := 0
			conn, err := safeDialContextWithDependencies(context.Background(), "tcp", "synthetic.example:443", resolver,
				func(_ context.Context, network, address string) (net.Conn, error) {
					dials++
					require.Equal(t, "tcp", network)
					require.Equal(t, "8.8.8.8:443", address)
					return nil, nil // mock success; never opens a socket
				})
			require.Nil(t, conn)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.Zero(t, dials)
			} else if tc.blocked {
				require.Error(t, err)
				require.Zero(t, dials, "all addresses must be checked before the first dial")
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, dials)
			}
		})
	}
}

func TestMonitorSSRFRebindingAndDialErrors(t *testing.T) {
	lookups := 0
	resolver := monitorResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
		lookups++
		ip := "8.8.8.8"
		if lookups > 1 {
			ip = "169.254.169.254"
		}
		return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
	})
	blocked, err := isPrivateOrLoopbackHostWithResolver(context.Background(), "synthetic.example", resolver)
	require.NoError(t, err)
	require.False(t, blocked)
	_, err = safeDialContextWithDependencies(context.Background(), "tcp", "synthetic.example:443", resolver,
		func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("rebound metadata address must not dial")
			return nil, nil
		})
	require.Error(t, err)
	require.Equal(t, 2, lookups)

	dialErr := errors.New("synthetic dial failure")
	resolver = monitorResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("2606:4700:4700::1111")}}, nil
	})
	var targets []string
	_, err = safeDialContextWithDependencies(context.Background(), "tcp", "synthetic.example:443", resolver,
		func(_ context.Context, _, address string) (net.Conn, error) {
			targets = append(targets, address)
			return nil, dialErr
		})
	require.ErrorIs(t, err, dialErr)
	require.Equal(t, []string{"8.8.8.8:443", "[2606:4700:4700::1111]:443"}, targets)
	targets = nil
	_, err = safeDialContextWithDependencies(context.Background(), "tcp", "synthetic.example:443", resolver,
		func(_ context.Context, _, address string) (net.Conn, error) {
			targets = append(targets, address)
			if len(targets) == 1 {
				return nil, dialErr
			}
			return nil, nil
		})
	require.NoError(t, err)
	require.Len(t, targets, 2, "an unreachable public address may fall back to another public address")

	_, err = safeDialContextWithDependencies(context.Background(), "tcp", "8.8.8.8:443",
		monitorResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
			t.Fatal("literal must not resolve")
			return nil, nil
		}),
		func(_ context.Context, _, address string) (net.Conn, error) {
			require.Equal(t, "8.8.8.8:443", address)
			return nil, nil
		})
	require.NoError(t, err)

	for _, address := range []string{"invalid-address", "localhost:443", "127.0.0.1:443", "[::1]:443"} {
		_, err = safeDialContextWithDependencies(context.Background(), "tcp", address,
			monitorResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
				t.Fatal("must not resolve blocked literal/hostname")
				return nil, nil
			}),
			func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("must not dial blocked address")
				return nil, nil
			})
		require.Error(t, err)
	}
}
