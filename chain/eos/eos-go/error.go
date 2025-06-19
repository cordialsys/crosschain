package eos

import (
	"fmt"
	"strings"
)

// APIError represents the errors as reported by the server
type APIError struct {
	Code        int    `json:"code"` // http code
	Message     string `json:"message"`
	ErrorStruct struct {
		Code    int              `json:"code"` // https://docs.google.com/spreadsheets/d/1uHeNDLnCVygqYK-V01CFANuxUwgRkNkrmeLm9MLqu9c/edit#gid=0
		Name    string           `json:"name"`
		What    string           `json:"what"`
		Details []APIErrorDetail `json:"details"`
	} `json:"error"`
}

func NewAPIError(httpCode int, msg string, e Error) *APIError {
	newError := &APIError{
		Code:    httpCode,
		Message: msg,
	}
	newError.ErrorStruct.Code = e.Code
	newError.ErrorStruct.Name = e.Name
	newError.ErrorStruct.What = msg
	newError.ErrorStruct.Details = []APIErrorDetail{
		APIErrorDetail{
			File:       "",
			LineNumber: 0,
			Message:    msg,
			Method:     e.Name,
		},
	}

	return newError
}

type APIErrorDetail struct {
	Message    string `json:"message"`
	File       string `json:"file"`
	LineNumber int    `json:"line_number"`
	Method     string `json:"method"`
}

func (e APIError) Error() string {
	msg := e.Message
	msg = fmt.Sprintf("%s: %s", msg, e.ErrorStruct.What)

	for _, detail := range e.ErrorStruct.Details {
		msg = fmt.Sprintf("%s: %s", msg, detail.Message)
	}

	return msg
}

// IsUnknowKeyError determines if the APIError is a 500 error
// with an `unknown key` message in at least one of the detail element.
// Some endpoint like `/v1/chain/get_account` returns a body in
// the form:
//
// ```
//
//	 {
//	 	"code": 500,
//	 	"message": "Internal Service Error",
//	 	"error": {
//	 		"code": 0,
//	 		"name": "exception",
//	 		"what": "unspecified",
//	 		"details": [
//			 		{
//			 			"message": "unknown key (<... redacted ...>): (0 eos.rex)",
//			 			"file": "http_plugin.cpp",
//			 			"line_number": 589,
//			 			"method": "handle_exception"
//			 		}
//	 		]
//	 	}
//	 }
//
// ```
//
// This will check if root code is a 500, that inner error code is 0 and there is
// a detail message starting with prefix `"unknown key"`.
func (e APIError) IsUnknownKeyError() bool {
	return e.Code == 500 &&
		e.ErrorStruct.Code == 0 &&
		e.hasDetailMessagePrefix("unknown key")
}

func (e APIError) hasDetailMessagePrefix(prefix string) bool {
	for _, detail := range e.ErrorStruct.Details {
		if strings.HasPrefix(detail.Message, prefix) {
			return true
		}
	}

	return false
}

type Error struct {
	Name string
	Code int
}

func (e Error) Error() string {
	return fmt.Sprintf("eos error: %q, code: %d", e.Name, e.Code)
}

var ErrUnspecifiedException = Error{"unspecified_exception_code", 3990000}
var ErrUnhandledException = Error{"unhandled_exception_code", 3990001}
var ErrTimeoutException = Error{"timeout_exception_code", 3990002}
var ErrFileNotFoundException = Error{"file_not_found_exception_code", 3990003}
var ErrParseErrorException = Error{"parse_error_exception_code", 3990004}
var ErrInvalidArgException = Error{"invalid_arg_exception_code", 3990005}
var ErrKeyNotFoundException = Error{"key_not_found_exception_code", 3990006}
var ErrBadCastException = Error{"bad_cast_exception_code", 3990007}
var ErrOutOfRangeException = Error{"out_of_range_exception_code", 3990008}
var ErrCanceledException = Error{"canceled_exception_code", 3990009}
var ErrAssertException = Error{"assert_exception_code", 3990010}
var ErrEOFException = Error{"eof_exception_code", 3990011}
var ErrStdException = Error{"std_exception_code", 3990013}
var ErrInvalidOperationException = Error{"invalid_operation_exception_code", 3990014}
var ErrUnknownHostException = Error{"unknown_host_exception_code", 3990015}
var ErrNullOptional = Error{"null_optional_code", 3990016}
var ErrUDTError = Error{"udt_error_code", 3990017}
var ErrAESError = Error{"aes_error_code", 3990018}
var ErrOverflow = Error{"overflow_code", 3990019}
var ErrUnderflow = Error{"underflow_code", 3990020}
var ErrDivideByZero = Error{"divide_by_zero_code", 3990021}
