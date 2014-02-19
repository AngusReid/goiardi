/*
 * Copyright (c) 2013-2014, Jeremy Bingham (<jbingham@gmail.com>)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package authentication

import (
	//"github.com/ctdk/goiardi/chef_crypto"
	"github.com/ctdk/goiardi/actor"
	"github.com/ctdk/goiardi/util"
	"net/http"
	"io"
	"io/ioutil"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"log"
	"strings"
	"regexp"
	"strconv"
)

func CheckHeader(user_id string, r *http.Request) (bool, util.Gerror) {
	user, err := actor.Get(user_id)
	_ = user
/*	if err != nil {
		gerr := util.Errorf(err.Error())
		gerr.SetStatus(http.StatusUnauthorized)
		return false, gerr
	} */
	contentHash := r.Header.Get("X-OPS-CONTENT-HASH")
	if contentHash == "" {
		gerr := util.Errorf("no content hash provided")
		gerr.SetStatus(http.StatusUnauthorized)
		return false, gerr
	}
	authTimestamp := r.Header.Get("x-ops-timestamp")
	if authTimestamp == "" {
		log.Printf("no timestamp, uh\n")
	} else {
		log.Printf("timestamp was: %s\n", authTimestamp)
	}
	var bodyStr string
	if r.Body == nil {
		bodyStr = ""
	} else {
		save := r.Body
		save, r.Body, err = drainBody(r.Body)
		if err != nil {
			gerr := util.Errorf("could not copy body")
			gerr.SetStatus(http.StatusInternalServerError)
			return false, gerr
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		bodyStr = buf.String()
		r.Body = save
	}
	h := sha1.New()
	io.WriteString(h, bodyStr)
	chkHash := base64.StdEncoding.EncodeToString(h.Sum(nil))
	log.Printf("content hash is: %s\ncalcnew hash is: %s\n", contentHash, chkHash)
	if chkHash != contentHash {
		gerr := util.Errorf("Content hash did not match hash of request body")
		gerr.SetStatus(http.StatusUnauthorized)
		return false, gerr
	}

	signedHeaders, sherr  := assembleSignedHeader(r)
	if sherr != nil {
		return false, sherr
	}
	log.Printf("The full signed encoded header is: %s\n", signedHeaders)

	return true, nil
}

// liberated from net/httputil
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
func drainBody(b io.ReadCloser) (r1, r2 io.ReadCloser, err error) {
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, nil, err
	}
	if err = b.Close(); err != nil {
		return nil, nil, err
	}
	return ioutil.NopCloser(&buf), ioutil.NopCloser(bytes.NewBuffer(buf.Bytes())), nil
}

func assembleSignedHeader(r *http.Request) (string, util.Gerror) {
	sHeadStore := make(map[int]string)
	authHeader := regexp.MustCompile(`(?i)^X-Ops-Authorization-(\d+)`)
	for k := range r.Header {
		if c := authHeader.FindStringSubmatch(k); c != nil {
			/* Have to put it into a map first, then sort, in case
			 * the headers don't come out in the right order */
			// skipping this error because we shouldn't even be
			// able to get here with something that won't be an
			// integer. Famous last words, I'm sure.
			i, _ := strconv.Atoi(c[1])
			sHeadStore[i] = r.Header.Get(k)
		}
	}
	if len(sHeadStore) == 0 {
		gerr := util.Errorf("No authentication headers found!")
		gerr.SetStatus(http.StatusUnauthorized)
		return "", gerr
	}

	sH := make([]string, len(sHeadStore))
	for k, v := range sHeadStore {
		sH[k - 1] = v
	}
	signedHeaders := strings.Join(sH, "")

	return signedHeaders, nil
}
