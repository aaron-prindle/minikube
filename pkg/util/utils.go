/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/version"
)

// Until endlessly loops the provided function until a message is received on the done channel.
// The function will wait the duration provided in sleep between function calls. Errors will be sent on provider Writer.
func Until(fn func() error, w io.Writer, name string, sleep time.Duration, done <-chan struct{}) {
	var exitErr error
	for {
		select {
		case <-done:
			return
		default:
			exitErr = fn()
			if exitErr == nil {
				fmt.Fprintf(w, Pad("%s: Exited with no errors.\n"), name)
			} else {
				fmt.Fprintf(w, Pad("%s: Exit with error: %v"), name, exitErr)
			}

			// wait provided duration before trying again
			time.Sleep(sleep)
		}
	}
}

func Pad(str string) string {
	return fmt.Sprintf("\n%s\n", str)
}

// If the file represented by path exists and
// readable, return true otherwise return false.
func CanReadFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}

	defer f.Close()

	return true
}

func Retry(attempts int, callback func() error) (err error) {
	return RetryAfter(attempts, callback, 0)
}

func RetryAfter(attempts int, callback func() error, d time.Duration) (err error) {
	m := MultiError{}
	for i := 0; i < attempts; i++ {
		err = callback()
		if err == nil {
			return nil
		}
		m.Collect(err)
		time.Sleep(d)
	}
	return m.ToError()
}

func GetLocalkubeDownloadURL(versionOrURL string, filename string) (string, error) {
	urlObj, err := url.Parse(versionOrURL)
	if err != nil {
		return "", err
	}
	if urlObj.IsAbs() {
		// scheme was specified in input, is a valid URI.
		// http.Get will catch unsupported schemes
		return versionOrURL, nil
	}
	if !strings.HasPrefix(versionOrURL, "v") {
		// no 'v' prefix in input, need to prepend it to version
		versionOrURL = "v" + versionOrURL
	}
	if _, err = semver.Make(strings.TrimPrefix(versionOrURL, version.VersionPrefix)); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s/%s", constants.LocalkubeDownloadURLPrefix, versionOrURL, filename), nil
}

type MultiError struct {
	Errors []error
}

func (m *MultiError) Collect(err error) {
	if err != nil {
		m.Errors = append(m.Errors, err)
	}
}

func (m MultiError) ToError() error {
	if len(m.Errors) == 0 {
		return nil
	}

	errStrings := []string{}
	for _, err := range m.Errors {
		errStrings = append(errStrings, err.Error())
	}
	return errors.New(strings.Join(errStrings, "\n"))
}

type ServiceContext struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

type Message struct {
	Message        string `json:"message"`
	ServiceContext `json:"serviceContext"`
}

func ReportError(err error) error {
	errMsg := fmt.Sprintf("%+v\n", err)
	errArray := strings.Split(errMsg, "\n")
	errOutput := []string{}
	// len check?
	errOutput = append(errOutput, errArray[0])
	for i := 1; i < len(errArray)-1; i += 2 {
		errOutput = append(errOutput, fmt.Sprintf("\tat %s (%s)", errArray[i],
			filepath.Base(errArray[i+1])))
	}
	errMsg = strings.Join(errOutput, "\n") + "\n"
	m := Message{errMsg, ServiceContext{"default", "v0.8.0"}}
	b, err := json.Marshal(m)
	fmt.Println(string(b))
	if err != nil {
		return errors.New(err.Error())
	}
	url := "https://clouderrorreporting.googleapis.com/v1beta1/projects/aprindle-vm/events:report?key=AIzaSyAicHoDP7bg78e60f60L08MTeL96HPvvSg"
	fmt.Println("URL:>", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return errors.New(err.Error())
	}
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New(err.Error())
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))
	return nil
}

func IsDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), nil
}
