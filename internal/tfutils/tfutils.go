package tfutils

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

const btpcliTag = "btpcli"

type any interface{}
type equalityPredicate[E any] func(E, E) bool

func ToBTPCLIParamsMap(a any) (map[string]string, error) {
	out := map[string]string{}

	v := reflect.ValueOf(a)

	if !v.IsValid() {
		return nil, fmt.Errorf("invalid value")
	}

	// unwrap until we reach a struct
	for {
		stop := false
		switch v.Kind() {
		case reflect.Pointer:
			v = v.Elem()
		case reflect.Interface:
			if v.IsNil() {
				return out, nil
			}
			v = v.Elem()
		default:
			stop = true
		}

		if stop {
			break
		}
	}

	if v.Kind() != reflect.Struct || !v.IsValid() {
		return nil, fmt.Errorf("unsupported type: %s", v.Kind())
	}

	for i := 0; i < v.NumField(); i++ {
		fieldProps := v.Type().Field(i)
		tagValue := fieldProps.Tag.Get(btpcliTag)

		if len(tagValue) == 0 {
			continue
		}

		field := v.FieldByName(fieldProps.Name)

		if !field.IsValid() {
			return nil, fmt.Errorf("invalid field")
		}

		var value string

		switch fieldProps.Type.String() {
		case "basetypes.StringValue":
			fieldVal := field.Interface().(types.String)

			if fieldVal.IsUnknown() || fieldVal.IsNull() {
				continue
			}

			value = fieldVal.ValueString()
		case "basetypes.BoolValue":
			fieldVal := field.Interface().(types.Bool)

			if fieldVal.IsUnknown() || fieldVal.IsNull() {
				continue
			}

			value = fmt.Sprintf("%v", fieldVal.ValueBool())
		case "bool":
			fieldVal := field.Interface().(bool)

			value = fmt.Sprintf("%v", fieldVal)
		case "string":
			fieldVal := field.Interface().(string)

			if fieldVal == "" {
				continue
			}

			value = fieldVal
		case "*string":
			if field.IsNil() {
				continue
			}

			value = field.Elem().Interface().(string)
		case "map[string][]string": // TODO would be nice to have `encodethisasjson` tag, instead of an explicit type mapping

			if field.IsNil() {
				continue
			}

			valueArr, err := json.Marshal(field.Interface())

			if err != nil {
				return nil, err
			}

			value = string(valueArr)
		default:
			return nil, fmt.Errorf("the type '%s' assigned to '%s' is not yet supported", fieldProps.Type.String(), tagValue)
		}

		out[tagValue] = value
	}

	return out, nil
}

// TODO This is a utility function to compute to be removed and to be added substructures in resource configurations.
// TODO This is required since terraform only computes required CRUD operations on resource level. Changes in inner
// TODO configurations need to be computed based on the state and plan data by the update operation of a provider.
// TODO Should the terraform plugin framework support this functionality in the future, e.g. as a part of the Set
// TODO datatype, we can remove this code.
func SetDifference[S ~[]E, E any](setA, setB S, isEqual equalityPredicate[E]) (result S) {
	for _, element := range setA {
		if !setContains(setB, element, isEqual) {
			result = append(result, element)
		}
	}
	return
}

func setContains[S ~[]E, E any](set S, element E, isEqual equalityPredicate[E]) bool {
	for _, setElement := range set {
		if isEqual(setElement, element) {
			return true
		}
	}
	return false
}
