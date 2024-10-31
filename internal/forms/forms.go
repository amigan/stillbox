package forms

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/jsontime"

	"github.com/araddon/dateparse"
)

var (
	ErrNotStruct  = errors.New("destination is not a struct")
	ErrNotPointer = errors.New("destination is not a pointer")
)

type options struct {
	tagOverride *string
	parseTimeIn *time.Location
	parseLocal  bool
	acceptBlank bool
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

func (o *options) iterFields(r *http.Request, rv reflect.Value) error {
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		tf := rt.Field(i)
		if !tf.IsExported() && !tf.Anonymous {
			continue
		}

		if f.Kind() == reflect.Struct && tf.Anonymous {
			err := o.iterFields(r, f)
			if err != nil {
				return err
			}
		}

		var tAr []string
		var formField string
		formTag, has := rt.Field(i).Tag.Lookup(o.Tag())
		if has {
			tAr = strings.Split(formTag, ",")
			formField = tAr[0]
		}
		if !has || formField == "-" {
			continue
		}

		fi := f.Interface()

		switch v := fi.(type) {
		case string, *string:
			s := r.Form.Get(formField)
			setVal(f, s != "" || o.acceptBlank, v, s)
		case int, uint, *int, *uint:
			ff := r.Form.Get(formField)
			val, set, err := o.parseInt(ff)
			if err != nil {
				return err
			}
			setVal(f, set, v, val)
		case float64:
			ff := r.Form.Get(formField)
			val, set, err := o.parseFloat64(ff)
			if err != nil {
				return err
			}
			setVal(f, set, v, val)
		case bool, *bool:
			ff := r.Form.Get(formField)
			val, set, err := o.parseBool(ff)
			if err != nil {
				return err
			}
			setVal(f, set, v, val)
		case []byte:
			file, hdr, err := r.FormFile(formField)
			if err != nil {
				return fmt.Errorf("get form file: %w", err)
			}

			nameField, hasFilename := rt.Field(i).Tag.Lookup("filenameField")
			if hasFilename {
				fnf := rv.FieldByName(nameField)
				if fnf == (reflect.Value{}) {
					panic(fmt.Errorf("filenameField '%s' does not exist", nameField))
				}

				fnf.SetString(hdr.Filename)
			}
			audioBytes, err := io.ReadAll(file)
			if err != nil {
				return fmt.Errorf("file read: %w", err)
			}

			f.SetBytes(audioBytes)
		case time.Time, *time.Time, jsontime.Time, *jsontime.Time:
			tval := r.Form.Get(formField)
			t, set, err := o.parseTime(tval)
			if err != nil {
				return err
			}
			setVal(f, set, v, t)
		case time.Duration, *time.Duration, jsontime.Duration, *jsontime.Duration:
			dval := r.Form.Get(formField)
			d, set, err := o.parseDuration(dval)
			if err != nil {
				return err
			}
			setVal(f, set, v, d)
		case []int:
			val := strings.Trim(r.Form.Get(formField), "[]")
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
			f.Set(reflect.ValueOf(ar))
		default:
			panic(fmt.Errorf("unsupported type %T", v))
		}
	}

	return nil
}

func setVal(setField reflect.Value, set bool, fv any, sv any) {
	if !set {
		return
	}

	rv := reflect.TypeOf(fv)
	svo := reflect.ValueOf(sv)

	if svo.CanConvert(rv) {
		svo = svo.Convert(rv)
	}

	if rv.Kind() == reflect.Ptr {
		svo = svo.Addr()
	}

	setField.Set(svo)
}

func Unmarshal(r *http.Request, dest any, opt ...Option) error {
	o := options{}
	for _, opt := range opt {
		opt(&o)
	}

	rv := reflect.ValueOf(dest)
	if k := rv.Kind(); k == reflect.Ptr {
		rv = rv.Elem()
	} else {
		return ErrNotPointer
	}

	if rv.Kind() != reflect.Struct {
		return ErrNotStruct
	}


	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		err := r.ParseForm()
		if err != nil {
			return fmt.Errorf("ParseForm: %w", err)
		}
	}

	return o.iterFields(r, rv)
}
