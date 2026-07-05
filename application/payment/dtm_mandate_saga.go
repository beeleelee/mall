package payment

import (
	"context"
	"fmt"

	"github.com/dtm-labs/client/dtmcli"

	"github.com/beeleelee/mall/domain/kernel"
)

type mandateSagaPayload struct {
	MandateID int64  `json:"mandate_id"`
	Token     string `json:"token"`
}

type sagaSubmitFn func(dtmServer, gid, cbURL string, payload mandateSagaPayload) error

type DTMMandateSaga struct {
	submitFn  sagaSubmitFn
	dtmServer string
	cbURL     string
	logger    kernel.Logger
}

func NewDTMMandateSaga(
	dtmServer string,
	callbackURL string,
	logger kernel.Logger,
	sagaSecret string,
) *DTMMandateSaga {
	return &DTMMandateSaga{
		submitFn:  makeMandateSubmit(sagaSecret),
		dtmServer: dtmServer,
		cbURL:     callbackURL,
		logger:    logger,
	}
}

func NewDTMMandateSagaWithSubmit(
	submitFn sagaSubmitFn,
	logger kernel.Logger,
) *DTMMandateSaga {
	return &DTMMandateSaga{
		submitFn: submitFn,
		logger:   logger,
	}
}

func makeMandateSubmit(secret string) sagaSubmitFn {
	return func(dtmServer, gid, cbURL string, payload mandateSagaPayload) error {
		saga := dtmcli.NewSaga(dtmServer, gid)
		if secret != "" {
			saga.BranchHeaders = map[string]string{"X-Saga-Secret": secret}
		}
		saga.Add(
			cbURL+"/api/v1/saga/mandate/execute",
			cbURL+"/api/v1/saga/payment/cancel",
			payload,
		)
		saga.Add(
			cbURL+"/api/v1/saga/mandate/settle",
			cbURL+"/api/v1/saga/mandate/rollback",
			map[string]any{"mandate_id": payload.MandateID},
		)
		return saga.Submit()
	}
}

func (s *DTMMandateSaga) Execute(ctx context.Context, mandateID kernel.ID, token string) error {
	gid := fmt.Sprintf("mt-%d", mandateID.Int64())
	payload := mandateSagaPayload{
		MandateID: mandateID.Int64(),
		Token:     token,
	}
	if err := s.submitFn(s.dtmServer, gid, s.cbURL, payload); err != nil {
		s.logger.Error(ctx, "dtm-mandate-saga: submit failed", err,
			kernel.Field("mandate_id", mandateID.String()),
			kernel.Field("gid", gid))
		return err
	}
	s.logger.Info(ctx, "dtm-mandate-saga: mandate executed and settled",
		kernel.Field("mandate_id", mandateID.String()),
		kernel.Field("gid", gid))
	return nil
}
