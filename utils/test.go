package utils

import (
	"reflect"
	"testing"
)

func InterfaceSlice(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic("InterfaceSlice() given a non-slice type")
	}

	ret := make([]interface{}, s.Len())

	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}

	return ret
}

type TestCollection struct {
	Collection []interface{}
	Cases      []TestCase
}

type TestCase struct {
	Name          string
	ExpectedValue interface{}
	ActualValue   interface{}
	ActualName    string
	Index         int
}

func AssertTestCase(t *testing.T, object interface{}, theCase TestCase) {
	if theCase.ActualName != "" {
		ref := reflect.ValueOf(object)
		fieldValue := reflect.Indirect(ref).FieldByName(theCase.ActualName).Interface()

		if !reflect.DeepEqual(theCase.ExpectedValue, fieldValue) {
			t.Errorf("expected %s='%+v', got '%+v' (@idx=%d)", theCase.ActualName, theCase.ExpectedValue, fieldValue, theCase.Index)
		}

	} else if theCase.ActualValue != nil && !reflect.DeepEqual(theCase.ExpectedValue, theCase.ActualValue) {
		t.Errorf("expected %s='%+v', got '%+v' (@idx=%d)", theCase.Name, theCase.ExpectedValue, theCase.ActualValue, theCase.Index)
	} else {
		t.Fatal("Invalid test case given!", theCase)
	}
}

func AssertTestCases(t *testing.T, collection TestCollection) {
	for _, theCase := range collection.Cases {
		AssertTestCase(t, collection.Collection[theCase.Index], theCase)
	}
}
