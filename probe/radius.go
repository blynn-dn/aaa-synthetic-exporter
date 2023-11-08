package probe

import (
	"context"
	"fmt"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"regexp"
	"time"
)

// CheckRadius attempt to perform a RADIUS operation
func CheckRadius(target string, secret string, username string, password string, operation string) Results {
	var results Results

	// create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// default to failed
	results.Success = false
	results.StatusCode = 0

	// if target doesn't specify a port, then append ":49"
	m, err := regexp.MatchString(":[0-9]+$", target)
	if err != nil {
		results.ErrorMessage = err.Error()
		return results
	}
	if !m {
		target += ":1812"
	}

	// use a goroutine to avoid blocking while performing the RADIUS check
	// create a channel for communicating the goroutine
	ch := make(chan Results)
	go func() {
		packet := radius.New(radius.CodeAccessRequest, []byte(secret))

		//goland:noinspection ALL
		rfc2865.UserName_SetString(packet, username)
		//goland:noinspection ALL
		rfc2865.UserPassword_SetString(packet, password)

		response, err := radius.Exchange(context.Background(), packet, target)
		if err != nil {
			results.ErrorMessage = err.Error()
		} else {
			results.Success = true
			results.StatusCode = 1
			results.Body = response
		}

		ch <- results
	}()

	// process the results or catch the context timeout exceeded event
	for {
		select {
		case <-ctx.Done():
			// the timeout was exceeded
			results.ErrorMessage = fmt.Sprintf("error: %s, while peforming %s on target: %s",
				ctx.Err(), operation, target)
			return results
		case resp := <-ch:
			return resp
		}
	}
}
