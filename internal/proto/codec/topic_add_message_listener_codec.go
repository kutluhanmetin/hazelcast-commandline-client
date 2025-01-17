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
	hztypes "github.com/hazelcast/hazelcast-go-client/types"
)

const (
	TopicAddMessageListenerCodecRequestMessageType  = int32(0x040200)
	TopicAddMessageListenerCodecResponseMessageType = int32(0x040201)

	TopicAddMessageListenerCodecEventTopicMessageType = int32(0x040202)

	TopicAddMessageListenerCodecRequestLocalOnlyOffset  = proto.PartitionIDOffset + proto.IntSizeInBytes
	TopicAddMessageListenerCodecRequestInitialFrameSize = TopicAddMessageListenerCodecRequestLocalOnlyOffset + proto.BooleanSizeInBytes

	TopicAddMessageListenerResponseResponseOffset      = proto.ResponseBackupAcksOffset + proto.ByteSizeInBytes
	TopicAddMessageListenerEventTopicPublishTimeOffset = proto.PartitionIDOffset + proto.IntSizeInBytes
	TopicAddMessageListenerEventTopicUuidOffset        = TopicAddMessageListenerEventTopicPublishTimeOffset + proto.LongSizeInBytes
)

// Subscribes to this topic. When someone publishes a message on this topic. onMessage() function of the given
// MessageListener is called. More than one message listener can be added on one instance.

func EncodeTopicAddMessageListenerRequest(name string, localOnly bool) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, TopicAddMessageListenerCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeBoolean(initialFrame.Content, TopicAddMessageListenerCodecRequestLocalOnlyOffset, localOnly)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(TopicAddMessageListenerCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, name)

	return clientMessage
}

func DecodeTopicAddMessageListenerResponse(clientMessage *proto.ClientMessage) hztypes.UUID {
	frameIterator := clientMessage.FrameIterator()
	initialFrame := frameIterator.Next()

	return DecodeUUID(initialFrame.Content, TopicAddMessageListenerResponseResponseOffset)
}

func HandleTopicAddMessageListener(clientMessage *proto.ClientMessage, handleTopicEvent func(item iserialization.Data, publishTime int64, uuid hztypes.UUID)) {
	messageType := clientMessage.Type()
	frameIterator := clientMessage.FrameIterator()
	if messageType == TopicAddMessageListenerCodecEventTopicMessageType {
		initialFrame := frameIterator.Next()
		publishTime := DecodeLong(initialFrame.Content, TopicAddMessageListenerEventTopicPublishTimeOffset)
		uuid := DecodeUUID(initialFrame.Content, TopicAddMessageListenerEventTopicUuidOffset)
		item := DecodeData(frameIterator)
		handleTopicEvent(item, publishTime, uuid)
	}
}
