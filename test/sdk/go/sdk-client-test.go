// Copyright 2019 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"log"
	"strconv"
	"time"

	pkgSdk "agones.dev/agones/pkg/sdk"
	goSdk "agones.dev/agones/sdks/go"
)

func main() {
	log.SetFlags(log.Lshortfile)
	log.Println("Client is starting")
	time.Sleep(100 * time.Millisecond)
	sdk, err := goSdk.NewSDK()
	if err != nil {
		log.Fatalf("Could not connect to sdk: %v\n", err)
	}

	c := make(chan string)

	once := true
	err = sdk.WatchGameServer(func(gs *pkgSdk.GameServer) {
		log.Println("Received GameServer update")
		log.Println(gs)
		uid := gs.ObjectMeta.Uid
		if once {
			c <- uid
			once = false
		}
	})
	if err != nil {
		log.Fatalf("Error on watch GameServer %s", err)
	}
	err = sdk.Ready()
	if err != nil {
		log.Fatalf("Could not send ready message %s", err)
	}
	if err = sdk.Reserve(5 * time.Second); err != nil {
		log.Fatalf("Could not send Reserve command: %s", err)
	}
	err = sdk.Allocate()
	if err != nil {
		log.Fatalf("Err sending allocate request %s", err)
	}
	err = sdk.Health()
	if err != nil {
		log.Fatalf("Could not send Health check: %s", err)
	}
	gs, err := sdk.GameServer()
	if err != nil {
		log.Fatalf("Could not get gameserver parameters: %s", err)
	}
	log.Println(gs)

	err = sdk.SetLabel("creationTimestamp", strconv.FormatInt(gs.ObjectMeta.CreationTimestamp, 10))
	if err != nil {
		log.Fatalf("Could not set label: %s", err)
	}
	if err != nil {
		log.Fatalf("Error received on watch gameserver %s", err)
	}
	uid := <-c
	err = sdk.SetAnnotation("UID", uid)
	if err != nil {
		log.Fatalf("Could not set annotation: %s", err)
	}
	err = sdk.Shutdown()
	if err != nil {
		log.Fatalf("Could not shutdown GameServer: %s", err)
	}
}
