// Mock dnf-json
//
// The purpose of this program is to return fake but expected responses to
// dnf-json depsolve and dump queries.  Tests should initialise a
// dnfjson.Solver and configure it to run this program via the SetDNFJSONPath()
// method.  This utility accepts queries and returns responses with the same
// structure as the dnf-json Python script.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/osbuild/images/internal/dnfjson"
)

func maybeFail(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}

func readRequest(r io.Reader) dnfjson.Request {
	j := json.NewDecoder(os.Stdin)
	j.DisallowUnknownFields()

	var req dnfjson.Request
	err := j.Decode(&req)
	maybeFail(err)
	return req
}

func readTestCase() string {
	if len(os.Args) < 2 {
		fail(errors.New("no test case specified"))
	}
	if len(os.Args) > 2 {
		fail(errors.New("invalid number of arguments: you must specify a test case"))
	}
	return os.Args[1]
}

func parseResponse(resp []byte, req dnfjson.Request) json.RawMessage {
	parsedResponse := make(map[string]json.RawMessage)
	err := json.Unmarshal(resp, &parsedResponse)
	maybeFail(err)

	if req.Command == "search" {
		// Search requests need to return results based on the search
		// The key to the search is a comma-separated list of the requested packages
		key := strings.Join(req.Arguments.Search.Packages, ",")

		// Extract the possible response map
		var searches map[string]json.RawMessage
		err = json.Unmarshal(parsedResponse["search"], &searches)
		maybeFail(err)

		if _, ok := searches[key]; !ok {
			fail(fmt.Errorf("search response map is missing key = %s", key))
		}
		return searches[key]
	} else {
		return parsedResponse[req.Command]
	}
}

func checkForError(msg json.RawMessage) bool {
	j := json.NewDecoder(bytes.NewReader(msg))
	j.DisallowUnknownFields()
	dnferror := new(dnfjson.Error)
	err := j.Decode(dnferror)
	return err == nil
}

func main() {
	testFilePath := readTestCase()

	req := readRequest(os.Stdin)

	testFile, err := os.Open(testFilePath)
	if err != nil {
		fail(fmt.Errorf("failed to open test file %q\n", testFilePath))
	}
	defer testFile.Close()
	response, err := io.ReadAll(testFile)
	if err != nil {
		fail(fmt.Errorf("failed to read test file %q\n", testFilePath))
	}

	res := parseResponse(response, req)

	if req.Command == "depsolve" {
		// add repo ID to packages
		// just use the first
		for _, repo := range req.Arguments.Repos {
			res = bytes.ReplaceAll(res, []byte("REPOID"), []byte(repo.ID))
			break
		}
	}
	fmt.Print(string(res))

	// check if we should return with error
	if checkForError(res) {
		os.Exit(1)
	}
}
