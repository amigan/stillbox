package policy

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"dynatron.me/x/stillbox/pkg/incidents/incstore"
	"dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/el-mike/restrict/v2"
	"github.com/google/uuid"
)

const (
	SubmitterEqualConditionType = "SUBMITTER_EQUAL"
	InMapConditionType          = "IN_MAP"
	CallInIncidentConditionType = "CALL_IN_INCIDENT"
	TGInIncidentConditionType   = "TG_IN_INCIDENT"
	SelfOrAdminConditionType    = "SELF_OR_ADMIN"
)

type TGInIncidentCondition struct {
	ID       string                    `json:"name,omitempty" yaml:"name,omitempty"`
	TG       *restrict.ValueDescriptor `json:"tg" yaml:"tg"`
	Incident *restrict.ValueDescriptor `json:"incident" yaml:"incident"`
}

func (*TGInIncidentCondition) Type() string {
	return TGInIncidentConditionType
}

func (c *TGInIncidentCondition) Check(r *restrict.AccessRequest) error {
	tgVID, err := c.TG.GetValue(r)
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

	tgID, isTGID := tgVID.(talkgroups.ID)
	if !isTGID {
		return restrict.NewConditionNotSatisfiedError(c, r, errors.New("tg ID is not TGID"))
	}

	// XXX: this should instead come from the access request context, for better reuse upstream
	tgm, err := incstore.FromCtx(ctx).TGsIn(ctx, incID)
	if err != nil {
		return restrict.NewConditionNotSatisfiedError(c, r, err)
	}

	if !tgm.Has(tgID) {
		return restrict.NewConditionNotSatisfiedError(c, r, fmt.Errorf(`tg "%v" not in incident "%v"`, tgID, incID))
	}

	return nil
}

type CallInIncidentCondition struct {
	ID       string                    `json:"name,omitempty" yaml:"name,omitempty"`
	Call     *restrict.ValueDescriptor `json:"call" yaml:"call"`
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

	inCall, err := incstore.FromCtx(ctx).CallIn(ctx, incID, callID)
	if err != nil {
		return restrict.NewConditionNotSatisfiedError(c, r, err)
	}

	if !inCall {
		return restrict.NewConditionNotSatisfiedError(c, r, fmt.Errorf(`call "%v" not in incident "%v"`, callID, incID))
	}

	return nil
}

type SubmitterEqualCondition struct {
	ID    string                    `json:"name,omitempty" yaml:"name,omitempty"`
	Left  *restrict.ValueDescriptor `json:"left" yaml:"left"`
	Right *restrict.ValueDescriptor `json:"right" yaml:"right"`
}

func (s *SubmitterEqualCondition) Type() string {
	return SubmitterEqualConditionType
}

func (c *SubmitterEqualCondition) Check(r *restrict.AccessRequest) error {
	left, err := c.Left.GetValue(r)
	if err != nil {
		return err
	}

	right, err := c.Right.GetValue(r)
	if err != nil {
		return err
	}

	lVal := reflect.ValueOf(left)
	rVal := reflect.ValueOf(right)

	// deref Left. this is the difference between us and EqualCondition
	for lVal.Kind() == reflect.Pointer {
		lVal = lVal.Elem()
	}

	if !lVal.IsValid() || !reflect.DeepEqual(rVal.Interface(), lVal.Interface()) {
		return restrict.NewConditionNotSatisfiedError(c, r, fmt.Errorf("values \"%v\" and \"%v\" are not equal", left, right))
	}

	return nil
}

func SubmitterEqualConditionFactory() restrict.Condition {
	return new(SubmitterEqualCondition)
}

type InMapCondition[K comparable, V any] struct {
	ID  string                    `json:"name,omitempty" yaml:"name,omitempty"`
	Key *restrict.ValueDescriptor `json:"key" yaml:"key"`
	Map *restrict.ValueDescriptor `json:"map" yaml:"map"`
}

func (s *InMapCondition[K, V]) Type() string {
	return SubmitterEqualConditionType
}

func InMapConditionFactory[K comparable, V any]() restrict.Condition {
	return new(InMapCondition[K, V])
}

func (c *InMapCondition[K, V]) Check(r *restrict.AccessRequest) error {
	cKey, err := c.Key.GetValue(r)
	if err != nil {
		return err
	}

	cMap, err := c.Map.GetValue(r)
	if err != nil {
		return err
	}

	key := cKey.(K)

	if _, in := cMap.(map[K]V)[key]; !in {
		return restrict.NewConditionNotSatisfiedError(c, r, fmt.Errorf("key '%v' not in map", key))
	}

	return nil
}

func init() {
	_ = restrict.RegisterConditionFactory(SubmitterEqualConditionType, SubmitterEqualConditionFactory)
}
