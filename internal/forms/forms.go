package forms

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/jsontypes"

	"github.com/araddon/dateparse"
)

var (
	ErrNotStruct   = errors.New("destination is not a struct")
	ErrNotPointer  = errors.New("destination is not a pointer")
	ErrContentType = errors.New("bad content type")
)

const (
	MaxMultipartMemory int64 = 1024 * 1024 // 1MB
)

type options struct {
	tagOverride        *string
	parseTimeIn        *time.Location
	parseLocal         bool
	acceptBlank        bool
	maxMultipartMemory int64
	defaultOmitEmpty   bool
}

type Option func(*options)

func WithParseTimeInTZ(l *time.Location) Option {
	return func(o *options) {
		o.parseTimeIn = l
	}
}

func WithParseLocalTime() Option {
	return func(o *options) {
		o.parseLocal = true
	}
}

func WithAcceptBlank() Option {
	return func(o *options) {
		o.acceptBlank = true
	}
}

func WithTag(t string) Option {
	return func(o *options) {
		o.tagOverride = &t
	}
}

func WithMaxMultipartSize(s int64) Option {
	return func(o *options) {
		o.maxMultipartMemory = s
	}
}

func WithOmitEmpty() Option {
	return func(o *options) {
		o.defaultOmitEmpty = true
	}
}

func (o *options) Tag() string {
	if o.tagOverride != nil {
		return *o.tagOverride
	}

	return "form"
}

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

func (o *options) iterFields(r *http.Request, destStruct reflect.Value) error {
	structType := destStruct.Type()
	for i := 0; i < destStruct.NumField(); i++ {
		destFieldVal := destStruct.Field(i)
		fieldType := structType.Field(i)
		if !fieldType.IsExported() && !fieldType.Anonymous {
			continue
		}

		if destFieldVal.Kind() == reflect.Struct && fieldType.Anonymous {
			err := o.iterFields(r, destFieldVal)
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
			panic(fmt.Errorf("unsupported type %T", v))
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

func Unmarshal(r *http.Request, dest any, opt ...Option) error {
	o := options{
		maxMultipartMemory: MaxMultipartMemory,
	}

	for _, opt := range opt {
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
	case "application/json":
		return json.NewDecoder(r.Body).Decode(dest)
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

	return o.iterFields(r, destVal)
}
