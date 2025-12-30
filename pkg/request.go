package vii

import "net/http"

//=====================================
// request / response helpers
//=====================================

func Param(r *http.Request, paramName string) string {
	return r.URL.Query().Get(paramName)
}

func ParamIs(r *http.Request, paramName string, valueToCheck string) bool {
	return r.URL.Query().Get(paramName) == valueToCheck
}
