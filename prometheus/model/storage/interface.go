// Copyright 2014 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"github.com/liticer/gclients/prometheus/model/labels"
)

// Series exposes a single time series and allows iterating over samples.
type Series interface {
	Labels
}

// Labels represents an item that has labels e.g. time series.
type Labels interface {
	// Labels returns the complete set of labels. For series it means all labels identifying the series.
	Labels() labels.Labels
}

type Warnings []error

// SeriesSet contains a set of series.
type SeriesSet interface {
	Next() bool
	// At returns full series. Returned series should be iterable even after Next is called.
	At() Series
	// The error that iteration as failed with.
	// When an error occurs, set cannot continue to iterate.
	Err() error
	// A collection of warnings for the whole set.
	// Warnings could be return even iteration has not failed with error.
	Warnings() Warnings
}
