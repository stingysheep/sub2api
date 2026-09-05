package service

import "context"

type BalanceSource string

const (
	BalanceSourceFree BalanceSource = "free"
	BalanceSourcePaid BalanceSource = "paid"
)

type BalanceSourceUserRepository interface {
	AdjustBalanceBySource(ctx context.Context, id int64, delta float64, source BalanceSource) (BalanceChange, error)
	SetBalanceBySource(ctx context.Context, id int64, value float64, source BalanceSource) (BalanceChange, error)
}

type balanceSourceContextKey struct{}

func WithBalanceSource(ctx context.Context, source BalanceSource) context.Context {
	if source != BalanceSourcePaid {
		source = BalanceSourceFree
	}
	return context.WithValue(ctx, balanceSourceContextKey{}, source)
}

func BalanceSourceFromContext(ctx context.Context) BalanceSource {
	if ctx != nil {
		if source, ok := ctx.Value(balanceSourceContextKey{}).(BalanceSource); ok && source == BalanceSourcePaid {
			return BalanceSourcePaid
		}
	}
	return BalanceSourceFree
}

func NormalizeBalanceSource(source BalanceSource) BalanceSource {
	if source == BalanceSourcePaid {
		return BalanceSourcePaid
	}
	return BalanceSourceFree
}
