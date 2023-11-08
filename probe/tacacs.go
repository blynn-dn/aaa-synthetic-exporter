package probe

import (
	"context"
	"fmt"
	tq "github.com/facebookincubator/tacquito"
	"net"
	"regexp"
	"time"
)

// CheckTacacs attempt to perform a TACACS operation
func CheckTacacs(target string, secret string, username string, password string, operation string) Results {
	var req *tq.Packet
	results := Results{}
	results.Success = false
	results.StatusCode = 0

	// create a context with a timeout
	// Note that tacquito does not implement nor expose a way to apply a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("target: %s, username: %s, operation: %s\n\n", target, username, operation)

	// if target doesn't specify a port, then append ":49"
	m, err := regexp.MatchString(":[0-9]+$", target)
	if err != nil {
		results.ErrorMessage = err.Error()
		return results
	}
	if !m {
		target += ":49"
	}

	// todo consider additional TACACS operations -- currently on authenticate is supported
	switch operation {
	case "authenticate":
		fmt.Printf("calling PARRuest: %s", target)

		// todo consider supporting CHAP -- currently on PAP is supported
		req = newPAPRequest(username, password)
	}

	// create a channel and go func to avoid blocking and to enforce a timeout if exceeded
	ch := make(chan Results)
	go func() {

		response, err := getResponse(tq.SetClientDialer("tcp", target, []byte(secret)), req)
		if err != nil {
			results.ErrorMessage = err.Error()
		} else {
			results.Body = response

			if response.Status == 1 {
				results.Success = true
				results.StatusCode = 1
			} else {
				results.ErrorMessage = fmt.Sprintf("error: %s", response.Status)
			}
		}

		ch <- results
	}()

	for {
		select {
		case <-ctx.Done():
			// the timeout was exceeded
			results.ErrorMessage = fmt.Sprintf("error: %s, while peforming %s on target: %s",
				ctx.Err(), operation, target)

			fmt.Printf("error: %v\n", results)
			return results
		case resp := <-ch:
			fmt.Printf("response: %v\n", results)
			return resp
		}
	}
}

// send a request and await a response
// this function may block forever and is meant to be called by a go func with a timer context
func getResponse(d tq.ClientOption, req *tq.Packet) (tq.AuthenReply, error) {
	var body tq.AuthenReply

	c, err := tq.NewClient(d)
	fmt.Printf("new client done\n")
	if err != nil {
		return body, err
	}
	defer c.Close()

	resp, err := c.Send(req)
	if err != nil {
		return body, err
	}

	if err := tq.Unmarshal(resp.Body, &body); err != nil {
		return body, err
	}

	return body, nil
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// generates a PAP request
func newPAPRequest(username string, password string) *tq.Packet {
	return tq.NewPacket(
		tq.SetPacketHeader(
			tq.NewHeader(
				tq.SetHeaderVersion(tq.Version{MajorVersion: tq.MajorVersion, MinorVersion: tq.MinorVersionOne}),
				tq.SetHeaderType(tq.Authenticate),
				tq.SetHeaderRandomSessionID(),
			),
		),
		tq.SetPacketBodyUnsafe(
			tq.NewAuthenStart(
				tq.SetAuthenStartType(tq.AuthenTypePAP),
				tq.SetAuthenStartAction(tq.AuthenActionLogin),
				tq.SetAuthenStartPrivLvl(tq.PrivLvl(1)),
				tq.SetAuthenStartPort(tq.AuthenPort("tty0")),
				tq.SetAuthenStartRemAddr(tq.AuthenRemAddr(GetLocalIP())),
				tq.SetAuthenStartUser(tq.AuthenUser(username)),
				tq.SetAuthenStartData(tq.AuthenData(password)),
			),
		),
	)
}

// helper: parse a response
func parseResponse(resp *tq.Packet) (string, error) {
	var body tq.AuthenReply
	if err := tq.Unmarshal(resp.Body, &body); err != nil {
		return "", err
	}
	fmt.Printf("%s", body)
	if body.Status == 1 {

	}
	return fmt.Sprintf("\n%+v\n", body), nil
}

// helper: print a response
func printPAPResponse(resp *tq.Packet) {
	fmt.Printf(parseResponse(resp))
}
