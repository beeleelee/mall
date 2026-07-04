package checkout

import (
	"context"

	appPayment "github.com/beeleelee/mall/application/payment"
	"github.com/beeleelee/mall/domain/kernel"
	domainPayment "github.com/beeleelee/mall/domain/payment"
)

type CheckoutMandateVerifier struct {
	paymentSvc     *domainPayment.PaymentService
	dtmSaga        *appPayment.DTMMandateSaga
	tokenValidator domainPayment.WalletTokenValidator
}

func NewCheckoutMandateVerifier(
	paymentSvc *domainPayment.PaymentService,
	dtmSaga *appPayment.DTMMandateSaga,
	tokenValidator domainPayment.WalletTokenValidator,
) *CheckoutMandateVerifier {
	return &CheckoutMandateVerifier{
		paymentSvc:     paymentSvc,
		dtmSaga:        dtmSaga,
		tokenValidator: tokenValidator,
	}
}

func (v *CheckoutMandateVerifier) VerifyAndExecute(ctx context.Context, mandateID, userID kernel.ID, amount int64) error {
	m, err := v.paymentSvc.GetMandate(ctx, mandateID)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInvalidArgument, "mandate verification failed: cannot retrieve mandate", err)
	}

	if m.UserID != userID {
		return kernel.NewDomainError(kernel.ErrPermissionDenied, "mandate does not belong to this user")
	}

	if m.Scope.MaxAmount < amount {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "mandate max amount is less than checkout total")
	}

	if m.HasExpired() {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "mandate has expired")
	}

	if err := v.dtmSaga.Execute(ctx, mandateID, "checkout-"+mandateID.String()); err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "mandate verification failed: cannot execute mandate saga", err)
	}

	return nil
}

func (v *CheckoutMandateVerifier) VerifyAndExecuteWithToken(ctx context.Context, mandateID, userID kernel.ID, amount int64, token, provider string) error {
	m, err := v.paymentSvc.GetMandate(ctx, mandateID)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInvalidArgument, "mandate verification failed: cannot retrieve mandate", err)
	}

	if m.UserID != userID {
		return kernel.NewDomainError(kernel.ErrPermissionDenied, "mandate does not belong to this user")
	}

	if m.Scope.MaxAmount < amount {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "mandate max amount is less than checkout total")
	}

	if m.HasExpired() {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "mandate has expired")
	}

	result, err := v.tokenValidator.ValidateToken(ctx, token, provider)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInvalidArgument, "mandate verification failed: wallet token validation failed", err)
	}

	if err := v.dtmSaga.Execute(ctx, mandateID, result.Token); err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "mandate verification failed: cannot execute mandate saga", err)
	}

	return nil
}
