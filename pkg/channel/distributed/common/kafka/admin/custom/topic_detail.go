/*
Copyright 2020 The Knative Authors

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

package custom

import (
	"github.com/Shopify/sarama"
)

//
//  This Golang Type can be used by third party implementers of the eventing-kafka
//  "custom" AdminClient Type when building their REST endpoints.  They are of
//  course free to use any language when developing their Sidecar REST endpoints,
//  but presumably most will use Go.
//
//  This is currently an exact copy of the Sarama model, but is provided to
//  de-couple the Sarama implementation.  This simplifies things for the
//  third party implementers and allows us to extend if necessary.
//

// Custom TopicDetail Struct
type TopicDetail struct {
	NumPartitions     int32              `json:"numPartitions"`
	ReplicationFactor int16              `json:"replicationFactor"`
	ReplicaAssignment map[int32][]int32  `json:"replicaAssignment,omitempty"`
	ConfigEntries     map[string]*string `json:"configEntries,omitempty"`
}

// Custom TopicDetail Constructor
func NewTopicDetail(numPartitions int32, replicationFactor int16, replicaAssignment map[int32][]int32, configEntries map[string]*string) *TopicDetail {
	return &TopicDetail{
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
		ReplicaAssignment: replicaAssignment,
		ConfigEntries:     configEntries,
	}
}

// Convert The Current Custom TopicDetail To A Sarama TopicDetail
func (c *TopicDetail) ToSaramaTopicDetail() *sarama.TopicDetail {
	return &sarama.TopicDetail{
		NumPartitions:     c.NumPartitions,
		ReplicationFactor: c.ReplicationFactor,
		ReplicaAssignment: c.ReplicaAssignment,
		ConfigEntries:     c.ConfigEntries,
	}
}

// Convert A Sarama TopicDetail To The Current Custom TopicDetail
func (c *TopicDetail) FromSaramaTopicDetail(topicDetail *sarama.TopicDetail) {
	if topicDetail != nil {
		c.NumPartitions = topicDetail.NumPartitions
		c.ReplicationFactor = topicDetail.ReplicationFactor
		c.ReplicaAssignment = topicDetail.ReplicaAssignment
		c.ConfigEntries = topicDetail.ConfigEntries
	} else {
		c.NumPartitions = 0
		c.ReplicationFactor = 0
		c.ReplicaAssignment = nil
		c.ConfigEntries = nil
	}
}
