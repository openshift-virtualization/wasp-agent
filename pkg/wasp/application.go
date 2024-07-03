/*
 * This file is part of the Wasp project
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
 *
 * Copyright 2023,Red Hat, Inc.
 *
 */
package wasp

import (
	"context"
	"flag"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"kubevirt.io/wasp/pkg/client"
	"kubevirt.io/wasp/pkg/informers"
	"kubevirt.io/wasp/pkg/log"
	eviction_controller "kubevirt.io/wasp/pkg/wasp/eviction-controller"
	"os"
	"strconv"
	"time"
)

type WaspApp struct {
	evictionController              *eviction_controller.EvictionController
	podInformer                     cache.SharedIndexInformer
	nodeInformer                    cache.SharedIndexInformer
	ctx                             context.Context
	maxMemoryOverCommitmentBytes    resource.Quantity
	cli                             client.WaspClient
	maxAverageSwapInPagesPerSecond  float32
	maxAverageSwapOutPagesPerSecond float32
	AverageWindowSizeSeconds        time.Duration
	waspNs                          string
	nodeName                        string
	fsRoot                          string
}

func Execute() {
	var err error
	flag.Parse()
	var app = WaspApp{}
	memoryOverCommitmentThreshold := os.Getenv("MEMORY_OVER_COMMITMENT_THRESHOLD")
	maxAverageSwapInPagesPerSecond := os.Getenv("MAX_AVERAGE_SWAP_IN_PAGES_PER_SECOND")
	maxAverageSwapOutPagesPerSecond := os.Getenv("MAX_AVERAGE_SWAP_OUT_PAGES_PER_SECOND")
	AverageWindowSizeSeconds := os.Getenv("AVERAGE_WINDOW_SIZE_SECONDS")
	app.nodeName = os.Getenv("NODE_NAME")
	app.fsRoot = os.Getenv("FSROOT")

	app.maxMemoryOverCommitmentBytes, err = resource.ParseQuantity(memoryOverCommitmentThreshold)
	if err != nil {
		panic(err)
	}

	AverageWindowSizeSecondsToConvert, err := strconv.Atoi(AverageWindowSizeSeconds)
	if err != nil {
		panic(err)
	}
	app.AverageWindowSizeSeconds = time.Duration(AverageWindowSizeSecondsToConvert) * time.Second

	maxAverageSwapInPagesPerSecondToConvert, err := strconv.Atoi(maxAverageSwapInPagesPerSecond)
	if err != nil {
		panic(err)
	}
	app.maxAverageSwapInPagesPerSecond = float32(maxAverageSwapInPagesPerSecondToConvert)

	maxSwapOutRateToConvert, err := strconv.Atoi(maxAverageSwapOutPagesPerSecond)
	if err != nil {
		panic(err)
	}
	app.maxAverageSwapOutPagesPerSecond = float32(maxSwapOutRateToConvert)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.ctx = ctx

	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		panic(err)
	}
	app.waspNs = string(nsBytes)
	app.cli, err = client.GetWaspClient()
	if err != nil {
		panic(err)
	}
	app.podInformer = informers.GetPodInformer(app.cli)
	app.nodeInformer = informers.GetNodeInformer(app.cli)

	log.Log.Infof("MEMORY_OVER_COMMITMENT_THRESHOLD:%v "+
		"MAX_AVERAGE_SWAP_IN_PAGES_PER_SECOND:%v "+
		"MAX_AVERAGE_SWAP_OUT_PAGES_PER_SECOND:%v "+
		"INTERVAL:%v "+
		"nodeName: %v "+
		"ns: %v "+
		"fsRoot: %v",
		app.maxMemoryOverCommitmentBytes,
		app.maxAverageSwapInPagesPerSecond,
		app.maxAverageSwapOutPagesPerSecond,
		app.AverageWindowSizeSeconds,
		app.nodeName,
		app.waspNs,
		app.fsRoot,
	)
	stop := ctx.Done()
	app.initEvictionController(stop)
	app.Run(stop)
}

func (waspapp *WaspApp) initEvictionController(stop <-chan struct{}) {
	waspapp.evictionController = eviction_controller.NewEvictionController(waspapp.cli,
		waspapp.podInformer,
		waspapp.nodeInformer,
		waspapp.nodeName,
		waspapp.maxAverageSwapInPagesPerSecond,
		waspapp.maxAverageSwapOutPagesPerSecond,
		waspapp.maxMemoryOverCommitmentBytes,
		waspapp.AverageWindowSizeSeconds,
		waspapp.waspNs,
		stop,
	)
}

func (waspapp *WaspApp) Run(stop <-chan struct{}) {
	go waspapp.podInformer.Run(stop)
	go waspapp.nodeInformer.Run(stop)

	if !cache.WaitForCacheSync(stop,
		waspapp.podInformer.HasSynced,
		waspapp.nodeInformer.HasSynced,
	) {
		klog.Warningf("failed to wait for caches to sync")
	}

	go func() {
		waspapp.evictionController.Run(waspapp.ctx)
	}()
	<-waspapp.ctx.Done()

}
