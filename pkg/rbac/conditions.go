package rbac

import (
	"fmt"
	"reflect"

	"github.com/el-mike/restrict/v2"
)

const (
	SubmitterEqualConditionType = "SUBMITTER_EQUAL"
	InMapConditionType          = "IN_MAP"
)

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

	keyVal := reflect.ValueOf(cKey)
	mapVal := reflect.ValueOf(cMap)

	key := keyVal.Interface().(K)

	if _, in := mapVal.Interface().(map[K]V)[key]; !in {
		return restrict.NewConditionNotSatisfiedError(c, r, fmt.Errorf("key '%v' not in map", key))
	}

	return nil
}

func init() {
	restrict.RegisterConditionFactory(SubmitterEqualConditionType, SubmitterEqualConditionFactory)
}
