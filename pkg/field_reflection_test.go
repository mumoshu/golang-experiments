package pkg

import (
	"reflect"
	"testing"
)

func TestFieldReflection(t *testing.T) {
	s := struct {
		Foo string `a1:"lorem ipsum"`
		Bar int    `a1:"a b" a2:"a b"`
	}{}

	val := reflect.ValueOf(&s).Elem()

	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		tagA1 := typeField.Tag.Get("a1")
		tagA2 := typeField.Tag.Get("a2")

		t.Logf("Field Name: %v, \t Field Value: %s,\t Tag value: a1=%s, a2=%s\n", typeField.Name, valueField, tagA1, tagA2)
	}

	check := func(i int, v reflect.Value, a1, a2 string) {
		t.Helper()

		gotV := val.Field(i)
		gotT := val.Type().Field(i)
		tagA1 := gotT.Tag.Get("a1")
		tagA2 := gotT.Tag.Get("a2")

		if !reflect.DeepEqual(v.Interface(), gotV.Interface()) {
			t.Errorf("want value %+v, got %+v", v, gotV)
		}

		if a1 != tagA1 {
			t.Errorf("want a1 tag %q, got %q", a1, tagA1)
		}

		if a2 != tagA2 {
			t.Errorf("want a2 tag %q, got %q", a2, tagA2)
		}
	}

	check(0, reflect.ValueOf(""), "lorem ipsum", "")
	check(1, reflect.ValueOf(0), "a b", "a b")
}
