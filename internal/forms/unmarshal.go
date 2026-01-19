package forms

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/jsontypes"

	"github.com/araddon/dateparse"
)

func (o *options) parseTime(s string, dpo ...dateparse.ParserOption) (t time.Time, set bool, err error) {
	if o.acceptBlank && s == "" {
		set = false
		return
	}

	if iv, err := strconv.Atoi(s); err == nil {
		return time.Unix(int64(iv), 0), true, nil
	}

	switch {
	case o.parseTimeIn != nil:
		t, err = dateparse.ParseIn(s, o.parseTimeIn, dpo...)
	case o.parseLocal:
		t, err = dateparse.ParseLocal(s, dpo...)
	default:
		t, err = dateparse.ParseAny(s, dpo...)
	}

	set = true

	return
}

func (o *options) parseBool(s string) (v bool, set bool, err error) {
	if o.acceptBlank && s == "" {
		set = false
		return
	}

	set = true

	v, err = strconv.ParseBool(s)
	if err != nil {
		return v, set, fmt.Errorf("parsebool('%s'): %w", s, err)
	}

	return
}

func (o *options) parseInt(s string) (v int, set bool, err error) {
	if o.acceptBlank && s == "" {
		set = false
		return
	}
	set = true

	v, err = strconv.Atoi(s)
	if err != nil {
		return v, set, fmt.Errorf("atoi('%s'): %w", s, err)
	}

	return
}

func (o *options) parseFloat64(s string) (v float64, set bool, err error) {
	if o.acceptBlank && s == "" {
		set = false
		return
	}
	set = true

	v, err = strconv.ParseFloat(s, 64)
	if err != nil {
		return v, set, fmt.Errorf("ParseFloat('%s'): %w", s, err)
	}

	return
}

func (o *options) parseDuration(s string) (v time.Duration, set bool, err error) {
	if o.acceptBlank && s == "" {
		set = false
		return
	}

	set = true

	v, err = time.ParseDuration(s)
	if err != nil {
		return v, set, fmt.Errorf("ParseDuration('%s'): %w", s, err)
	}

	return
}

var typeOfByteSlice = reflect.TypeOf([]byte(nil))

func (o *options) unmIterFields(r *http.Request, destStruct reflect.Value) error {
	textUnmarshaler := reflect.TypeFor[encoding.TextUnmarshaler]()
	structType := destStruct.Type()
	for i := range destStruct.NumField() {
		destFieldVal := destStruct.Field(i)
		fieldType := structType.Field(i)
		if !fieldType.IsExported() && !fieldType.Anonymous {
			continue
		}

		if destFieldVal.Kind() == reflect.Struct && fieldType.Anonymous {
			err := o.unmIterFields(r, destFieldVal)
			if err != nil {
				return err
			}
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
			if slices.Contains(tAr[1:], "omitempty") {
				omitEmpty = true
			}
		}

		if !has || formField == "-" {
			continue
		}

		destFieldIntf := destFieldVal.Interface()

		if destFieldVal.Kind() == reflect.Slice && destFieldVal.Type() == typeOfByteSlice {
			file, hdr, err := r.FormFile(formField)
			if err != nil {
				return fmt.Errorf("get form file: %w", err)
			}

			nameField, hasFilename := structType.Field(i).Tag.Lookup("filenameField")
			if hasFilename {
				fnf := destStruct.FieldByName(nameField)
				if fnf == (reflect.Value{}) {
					panic(fmt.Errorf("filenameField '%s' does not exist", nameField))
				}

				fnf.SetString(hdr.Filename)
			}
			audioBytes, err := io.ReadAll(file)
			if err != nil {
				return fmt.Errorf("file read: %w", err)
			}

			destFieldVal.SetBytes(audioBytes)

			continue
		}

		if !r.Form.Has(formField) && omitEmpty {
			continue
		}

		ff := r.Form.Get(formField)

		destFieldType := destFieldVal.Type()

		switch v := destFieldIntf.(type) {
		case string, *string:
			setVal(destFieldVal, ff != "" || o.acceptBlank, ff)
		case int, uint, *int, *uint:
			val, set, err := o.parseInt(ff)
			if err != nil {
				return err
			}
			setVal(destFieldVal, set, val)
		case float64:
			val, set, err := o.parseFloat64(ff)
			if err != nil {
				return err
			}
			setVal(destFieldVal, set, val)
		case bool, *bool:
			val, set, err := o.parseBool(ff)
			if err != nil {
				return err
			}
			setVal(destFieldVal, set, val)
		case time.Time, *time.Time, jsontypes.Time, *jsontypes.Time:
			t, set, err := o.parseTime(ff)
			if err != nil {
				return err
			}
			setVal(destFieldVal, set, t)
		case time.Duration, *time.Duration, jsontypes.Duration, *jsontypes.Duration:
			d, set, err := o.parseDuration(ff)
			if err != nil {
				return err
			}
			setVal(destFieldVal, set, d)
		case []int:
			val := strings.Trim(ff, "[]")
			if val == "" && o.acceptBlank {
				continue
			}
			vals := strings.Split(val, ",")
			ar := make([]int, 0, len(vals))
			for _, v := range vals {
				i, err := strconv.Atoi(v)
				if err == nil {
					ar = append(ar, i)
				}
			}
			destFieldVal.Set(reflect.ValueOf(ar))
		default:
			if destFieldType.Kind() == reflect.Slice && reflect.PointerTo(destFieldType.Elem()).Implements(textUnmarshaler) {
				val := strings.Trim(ff, "[]")
				if val == "" && o.acceptBlank {
					continue
				}

				vals := strings.Split(val, ",")
				elemType := destFieldType.Elem()
				sliceVal := reflect.MakeSlice(destFieldType, len(vals), len(vals))

				for i, tElem := range vals {
					newElem := reflect.New(elemType)
					tum := newElem.Interface().(encoding.TextUnmarshaler)
					err := tum.UnmarshalText([]byte(tElem))
					if err != nil {
						return err
					}
					sliceVal.Index(i).Set(newElem.Elem())
				}

				destFieldVal.Set(sliceVal)

				continue
			}

			if reflect.PointerTo(destFieldType).Implements(textUnmarshaler) {
				tum := destFieldVal.Addr().Interface().(encoding.TextUnmarshaler)
				err := tum.UnmarshalText([]byte(ff))
				if err != nil {
					return err
				}

				continue
			}
			for destFieldType.Kind() == reflect.Ptr {
				destFieldType = destFieldType.Elem()
			}
			if reflect.ValueOf(ff).CanConvert(destFieldType) {
				setVal(destFieldVal, ff != "" || o.acceptBlank, ff)
			} else {
				panic(fmt.Errorf("unsupported type %T", v))
			}
		}
	}

	return nil
}

func setVal(destFieldVal reflect.Value, set bool, src any) {
	if !set {
		return
	}

	destType := destFieldVal.Type()
	srcVal := reflect.ValueOf(src)

	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}

	if destType.Kind() == reflect.Ptr {
		if !srcVal.CanAddr() {
			if srcVal.CanConvert(destType.Elem()) {
				srcVal = srcVal.Convert(destType.Elem())
			}
			copy := reflect.New(srcVal.Type())
			copy.Elem().Set(srcVal)
			srcVal = copy
		}
	} else if srcVal.CanConvert(destFieldVal.Type()) {
		srcVal = srcVal.Convert(destFieldVal.Type())
	}

	destFieldVal.Set(srcVal)
}

func Unmarshal(r *http.Request, dest any, opts ...Option) error {
	o := options{
		maxMultipartMemory: MaxMultipartMemory,
	}

	for _, opt := range opts {
		opt(&o)
	}

	contentType := strings.Split(r.Header.Get("Content-Type"), ";")[0]

	switch contentType {
	case "multipart/form-data":
		err := r.ParseMultipartForm(o.maxMultipartMemory)
		if err != nil {
			return fmt.Errorf("ParseForm: %w", err)
		}

		return o.unmarshalForm(r, dest)
	case "application/x-www-form-urlencoded":
		err := r.ParseForm()
		if err != nil {
			return fmt.Errorf("ParseForm: %w", err)
		}
		return o.unmarshalForm(r, dest)
	case "application/json", "":
		err := json.NewDecoder(r.Body).Decode(dest)
		switch err {
		case io.EOF:
			if o.acceptEmptyBody {
				return nil
			}
			fallthrough
		default:
			return err
		}
	}

	return ErrContentType
}

func (o *options) unmarshalForm(r *http.Request, dest any) error {
	destVal := reflect.ValueOf(dest)
	if k := destVal.Kind(); k == reflect.Ptr {
		destVal = destVal.Elem()
	} else {
		return ErrNotPointer
	}

	if destVal.Kind() != reflect.Struct {
		return ErrNotStruct
	}

	return o.unmIterFields(r, destVal)
}
