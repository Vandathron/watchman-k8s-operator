package utils

import (
	"fmt"
	"github.com/vandathron/watchman/internal/loghandler"
	"reflect"
)

// RecordChanges compares non(struct, func, chan) fields of old and new. Records changes
// by adding the difference to data.
func RecordChanges(old, new interface{}, prefix string, data *loghandler.Data) error {
	oldValue := reflect.ValueOf(old)
	newValue := reflect.ValueOf(new)

	if oldValue.Type() != newValue.Type() {
		return fmt.Errorf("old and new types not the same")
	}

	if oldValue.Kind() != reflect.Struct {
		return fmt.Errorf("must be a struct")
	}

	for i := 0; i < oldValue.NumField(); i++ {
		oldField := oldValue.Field(i)
		if oldField.Kind() == reflect.Struct || oldField.Kind() == reflect.Func || oldField.Kind() == reflect.Chan {
			continue
		}

		structField := oldValue.Type().Field(i)
		if structField.IsExported() {
			continue
		}
		newField := newValue.Field(i)

		if structField.Type.Kind() == reflect.Ptr {
			if !reflect.DeepEqual(oldField.Elem().Interface(), newField.Elem().Interface()) {
				data.AddField(fmt.Sprintf("%s%s", prefix, ""), fmt.Sprintf("%v", newField.Elem().Interface()))
			}
			continue
		}

		if !reflect.DeepEqual(oldField.Interface(), newField.Interface()) {
			data.AddField(fmt.Sprintf("%s%s", prefix, ""), fmt.Sprintf("%v", newField.Interface()))
		}
	}

	return nil
}
