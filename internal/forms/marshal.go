package forms

import (
	"fmt"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func Marshal(src any, dest *multipart.Writer, opts ...Option) error {
	o := options{}

	for _, opt := range opts {
		opt(&o)
	}

	return o.marshalMultipartForm(src, dest)
}

func (o *options) marshalMultipartForm(src any, dest *multipart.Writer) error {
	srcVal := reflect.ValueOf(src)
	if k := srcVal.Kind(); k == reflect.Ptr {
		srcVal = srcVal.Elem()
	}

	if srcVal.Kind() != reflect.Struct {
		return ErrNotStruct
	}

	return o.marIterFields(srcVal, dest)
}

func (o *options) marIterFields(srcVal reflect.Value, dest *multipart.Writer) error {
	structType := srcVal.Type()
	for i := 0; i < structType.NumField(); i++ {
		srcFieldVal := srcVal.Field(i)
		fieldType := structType.Field(i)
		if !fieldType.IsExported() && !fieldType.Anonymous {
			continue
		}

		if srcFieldVal.Kind() == reflect.Struct && fieldType.Anonymous {
			continue
		}

		var tAr []string
		var formField string
		var omitEmpty bool
		if o.defaultOmitEmpty {
			omitEmpty = true
		}

		formTag, has := structType.Field(i).Tag.Lookup(o.Tag())
		if has {
			tAr = strings.Split(formTag, ",")
			formField = tAr[0]
			for _, v := range tAr[1:] {
				if v == "omitempty" {
					omitEmpty = true
					break
				}
			}
		}

		if !has || formField == "-" {
			continue
		}

		if srcFieldVal.Kind() == reflect.Ptr {
			srcFieldVal = srcFieldVal.Elem()
			if srcFieldVal == (reflect.Value{}) || srcFieldVal.IsZero() {
				continue
			}
		}

		srcFieldIntf := srcFieldVal.Interface()
		if srcFieldVal.Kind() == reflect.Slice && srcFieldVal.Type() == typeOfByteSlice {
			nameField, hasFilename := structType.Field(i).Tag.Lookup("filenameField")
			fileName := ""
			if hasFilename {
				fnf := srcVal.FieldByName(nameField)
				if fnf == (reflect.Value{}) {
					panic(fmt.Errorf("filenameField '%s' does not exist", nameField))
				}

				fileName = fnf.String()
			}

			fw, err := dest.CreateFormFile(formField, fileName)
			if err != nil {
				return fmt.Errorf("form marshal: createFormFile: %w", err)
			}

			_, err = fw.Write(srcFieldVal.Bytes())
			if err != nil {
				return fmt.Errorf("form marshal: write file: %w", err)
			}

			continue
		}

		if srcFieldVal.IsZero() && omitEmpty {
			continue
		}

		var val string
		switch v := srcFieldIntf.(type) {
		case []string:
			if omitEmpty && len(v) == 0 {
				continue
			}
			val = "[" + strings.Join(v, ",") + "]"
		case []int:
			if omitEmpty && len(v) == 0 {
				continue
			}

			sl := make([]string, len(v))
			for i := range v {
				sl[i] = strconv.Itoa(v[i])
			}

			val = "[" + strings.Join(sl, ",") + "]"
		case time.Time:
			val = strconv.Itoa(int(v.Unix()))
		default:
			val = fmt.Sprint(v)
		}

		err := dest.WriteField(formField, val)
		if err != nil {
			return fmt.Errorf("marshal field '%s': %w", formField, err)
		}
	}

	return nil
}
