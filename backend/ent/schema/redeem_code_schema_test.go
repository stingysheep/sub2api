package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedeemCodeBalanceSourceValidator(t *testing.T) {
	var validators []func(string) error
	for _, entField := range (RedeemCode{}).Fields() {
		if entField.Descriptor().Name != "balance_source" {
			continue
		}
		for _, candidate := range entField.Descriptor().Validators {
			validator, ok := candidate.(func(string) error)
			require.True(t, ok)
			validators = append(validators, validator)
		}
	}
	require.Len(t, validators, 2)
	validator := func(value string) error {
		for _, candidate := range validators {
			if err := candidate(value); err != nil {
				return err
			}
		}
		return nil
	}

	require.NoError(t, validator("free"))
	require.NoError(t, validator("paid"))
	require.Error(t, validator("unknown"))
}
