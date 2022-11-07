package hwrap

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
)

const (
	strContext        = "context.Context"
	strRequest        = "*http.Request"
	strResponseWriter = "http.ResponseWriter"
	strError          = "error"
	strString         = "string"
)

var ErrorUnAuthorized = fmt.Errorf("UnAuthorized")

type DecoderFunc func(*http.Request, any) error

type AuthenFunc func(*http.Request) (*http.Request, error)

type ValidateFunc func(any) error

type Broker struct {
	fInV     reflect.Value
	fInT     reflect.Type
	f        any
	fIn      Arguments
	fOut     Arguments
	decode   DecoderFunc
	authen   AuthenFunc
	validate ValidateFunc
}

func (br *Broker) HandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fArgs, err := br.inputs(w, r)
		if err != nil {
			responseError(w, err)
			return
		}
		fRet := br.fInV.Call(fArgs)
		response(w, fRet)
	}
}

func (br *Broker) inputs(w http.ResponseWriter, r *http.Request) ([]reflect.Value, error) {
	rvs := []reflect.Value{}

	r, err := br.authen(r)
	if err != nil {
		return nil, ErrorUnAuthorized
	}

	for _, i := range br.fIn {
		switch i.String() {
		case strContext:
			rvs = append(rvs, reflect.ValueOf(r.Context()))
		case strRequest:
			rvs = append(rvs, reflect.ValueOf(r))
		case strResponseWriter:
			rvs = append(rvs, reflect.ValueOf(w))
		default:
			val := reflect.New(i)
			err := br.decode(r, val.Interface())
			if err != nil {
				return nil, err
			}
			rvs = appendValOrPtr(rvs, i.Kind(), val)
		}
	}
	return rvs, nil
}

func appendValOrPtr(rvs []reflect.Value, k reflect.Kind, rv reflect.Value) []reflect.Value {
	if k == reflect.Ptr {
		return append(rvs, rv)
	}
	return append(rvs, rv.Elem())
}

func response(w http.ResponseWriter, rr []reflect.Value) {
	switch len(rr) {
	case 0:
		w.WriteHeader(http.StatusOK)
		return
	case 1:
		switch rr[0].Type().String() {
		case strError:
			responseError(w, rr[0].Interface().(error))
		default:
			responseOk(w, rr[0])
		}
	case 2:
		if !rr[1].IsNil() {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		if rr[0].Type().String() == strString {
			io.WriteString(w, rr[0].Interface().(string))
			return
		}
	}
}

func responseOk(w http.ResponseWriter, v reflect.Value) {
	w.WriteHeader(http.StatusOK)
	switch v.Type().String() {
	case strString:
		io.WriteString(w, v.Interface().(string))
	default:
		_ = json.NewEncoder(w).Encode(v.Interface())
	}

}

func responseError(w http.ResponseWriter, err error) {
	if err == ErrorUnAuthorized {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": err.Error()})
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": err.Error()})
}
