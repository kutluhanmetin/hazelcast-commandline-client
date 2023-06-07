/*
* Copyright (c) 2008-2023, Hazelcast, Inc. All Rights Reserved.
*
* Licensed under the Apache License, Version 2.0 (the "License")
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
* http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package codec

import (
	iserialization "github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"
)

const (
	MultiMapLockCodecRequestMessageType  = int32(0x021000)
	MultiMapLockCodecResponseMessageType = int32(0x021001)

	MultiMapLockCodecRequestThreadIdOffset    = proto.PartitionIDOffset + proto.IntSizeInBytes
	MultiMapLockCodecRequestTtlOffset         = MultiMapLockCodecRequestThreadIdOffset + proto.LongSizeInBytes
	MultiMapLockCodecRequestReferenceIdOffset = MultiMapLockCodecRequestTtlOffset + proto.LongSizeInBytes
	MultiMapLockCodecRequestInitialFrameSize  = MultiMapLockCodecRequestReferenceIdOffset + proto.LongSizeInBytes
)

// Acquires the lock for the specified key for the specified lease time. After the lease time, the lock will be
// released. If the lock is not available, then the current thread becomes disabled for thread scheduling
// purposes and lies dormant until the lock has been acquired. Scope of the lock is for this map only. The acquired
// lock is only for the key in this map.Locks are re-entrant, so if the key is locked N times, then it should be
// unlocked N times before another thread can acquire it.

func EncodeMultiMapLockRequest(name string, key iserialization.Data, threadId int64, ttl int64, referenceId int64) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(true)

	initialFrame := proto.NewFrameWith(make([]byte, MultiMapLockCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeLong(initialFrame.Content, MultiMapLockCodecRequestThreadIdOffset, threadId)
	EncodeLong(initialFrame.Content, MultiMapLockCodecRequestTtlOffset, ttl)
	EncodeLong(initialFrame.Content, MultiMapLockCodecRequestReferenceIdOffset, referenceId)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(MultiMapLockCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, name)
	EncodeData(clientMessage, key)

	return clientMessage
}