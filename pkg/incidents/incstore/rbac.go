package incstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/el-mike/restrict/v2"
	"github.com/google/uuid"
)

const (
	CallInIncidentConditionType = "CALL_IN_INCIDENT"
)

type CallInIncidentCondition struct {
	ID    string                    `json:"name,omitempty" yaml:"name,omitempty"`
	Call *restrict.ValueDescriptor `json:"call" yaml:"call"`
	Incident *restrict.ValueDescriptor `json:"incident" yaml:"incident"`
}

func (*CallInIncidentCondition) Type() string {
	return CallInIncidentConditionType
}

func (c *CallInIncidentCondition) Check(r *restrict.AccessRequest) error {
	callVID, err := c.Call.GetValue(r)
	if err != nil {
		return err
	}

	incVID, err := c.Incident.GetValue(r)
	if err != nil {
		return err
	}

	ctx, hasCtx := r.Context["ctx"].(context.Context)
	if !hasCtx {
		return restrict.NewConditionNotSatisfiedError(c, r, fmt.Errorf("no context provided"))
	}

	incID, isUUID := incVID.(uuid.UUID)
	if !isUUID {
		return restrict.NewConditionNotSatisfiedError(c, r, errors.New("incident ID is not UUID"))
	}

	callID, isUUID := callVID.(uuid.UUID)
	if !isUUID {
		return restrict.NewConditionNotSatisfiedError(c, r, errors.New("call ID is not UUID"))
	}

	incs := FromCtx(ctx)
	inCall, err := incs.CallIn(ctx, incID, incID)
	if err != nil {
		return restrict.NewConditionNotSatisfiedError(c, r, err)
	}

	if !inCall {
		return restrict.NewConditionNotSatisfiedError(c, r, fmt.Errorf(`incident "%v" not in call "%v"`, incID, callID))
	}

	return nil
}


