// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v4.25.3
// source: blobstore_service.proto

package blobstore

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type BlobstoreServiceError_ErrorCode int32

const (
	BlobstoreServiceError_OK                        BlobstoreServiceError_ErrorCode = 0
	BlobstoreServiceError_INTERNAL_ERROR            BlobstoreServiceError_ErrorCode = 1
	BlobstoreServiceError_URL_TOO_LONG              BlobstoreServiceError_ErrorCode = 2
	BlobstoreServiceError_PERMISSION_DENIED         BlobstoreServiceError_ErrorCode = 3
	BlobstoreServiceError_BLOB_NOT_FOUND            BlobstoreServiceError_ErrorCode = 4
	BlobstoreServiceError_DATA_INDEX_OUT_OF_RANGE   BlobstoreServiceError_ErrorCode = 5
	BlobstoreServiceError_BLOB_FETCH_SIZE_TOO_LARGE BlobstoreServiceError_ErrorCode = 6
	BlobstoreServiceError_ARGUMENT_OUT_OF_RANGE     BlobstoreServiceError_ErrorCode = 8
	BlobstoreServiceError_INVALID_BLOB_KEY          BlobstoreServiceError_ErrorCode = 9
)

// Enum value maps for BlobstoreServiceError_ErrorCode.
var (
	BlobstoreServiceError_ErrorCode_name = map[int32]string{
		0: "OK",
		1: "INTERNAL_ERROR",
		2: "URL_TOO_LONG",
		3: "PERMISSION_DENIED",
		4: "BLOB_NOT_FOUND",
		5: "DATA_INDEX_OUT_OF_RANGE",
		6: "BLOB_FETCH_SIZE_TOO_LARGE",
		8: "ARGUMENT_OUT_OF_RANGE",
		9: "INVALID_BLOB_KEY",
	}
	BlobstoreServiceError_ErrorCode_value = map[string]int32{
		"OK":                        0,
		"INTERNAL_ERROR":            1,
		"URL_TOO_LONG":              2,
		"PERMISSION_DENIED":         3,
		"BLOB_NOT_FOUND":            4,
		"DATA_INDEX_OUT_OF_RANGE":   5,
		"BLOB_FETCH_SIZE_TOO_LARGE": 6,
		"ARGUMENT_OUT_OF_RANGE":     8,
		"INVALID_BLOB_KEY":          9,
	}
)

func (x BlobstoreServiceError_ErrorCode) Enum() *BlobstoreServiceError_ErrorCode {
	p := new(BlobstoreServiceError_ErrorCode)
	*p = x
	return p
}

func (x BlobstoreServiceError_ErrorCode) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (BlobstoreServiceError_ErrorCode) Descriptor() protoreflect.EnumDescriptor {
	return file_blobstore_service_proto_enumTypes[0].Descriptor()
}

func (BlobstoreServiceError_ErrorCode) Type() protoreflect.EnumType {
	return &file_blobstore_service_proto_enumTypes[0]
}

func (x BlobstoreServiceError_ErrorCode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use BlobstoreServiceError_ErrorCode.Descriptor instead.
func (BlobstoreServiceError_ErrorCode) EnumDescriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{0, 0}
}

type BlobstoreServiceError struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *BlobstoreServiceError) Reset() {
	*x = BlobstoreServiceError{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BlobstoreServiceError) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlobstoreServiceError) ProtoMessage() {}

func (x *BlobstoreServiceError) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlobstoreServiceError.ProtoReflect.Descriptor instead.
func (*BlobstoreServiceError) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{0}
}

type CreateUploadURLRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SuccessPath               string  `protobuf:"bytes,1,opt,name=success_path,json=successPath,proto3" json:"success_path,omitempty"`
	MaxUploadSizeBytes        *int64  `protobuf:"varint,2,opt,name=max_upload_size_bytes,json=maxUploadSizeBytes,proto3,oneof" json:"max_upload_size_bytes,omitempty"`
	MaxUploadSizePerBlobBytes *int64  `protobuf:"varint,3,opt,name=max_upload_size_per_blob_bytes,json=maxUploadSizePerBlobBytes,proto3,oneof" json:"max_upload_size_per_blob_bytes,omitempty"`
	GsBucketName              *string `protobuf:"bytes,4,opt,name=gs_bucket_name,json=gsBucketName,proto3,oneof" json:"gs_bucket_name,omitempty"`
	UrlExpiryTimeSeconds      *int32  `protobuf:"varint,5,opt,name=url_expiry_time_seconds,json=urlExpiryTimeSeconds,proto3,oneof" json:"url_expiry_time_seconds,omitempty"`
}

func (x *CreateUploadURLRequest) Reset() {
	*x = CreateUploadURLRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateUploadURLRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateUploadURLRequest) ProtoMessage() {}

func (x *CreateUploadURLRequest) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateUploadURLRequest.ProtoReflect.Descriptor instead.
func (*CreateUploadURLRequest) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{1}
}

func (x *CreateUploadURLRequest) GetSuccessPath() string {
	if x != nil {
		return x.SuccessPath
	}
	return ""
}

func (x *CreateUploadURLRequest) GetMaxUploadSizeBytes() int64 {
	if x != nil && x.MaxUploadSizeBytes != nil {
		return *x.MaxUploadSizeBytes
	}
	return 0
}

func (x *CreateUploadURLRequest) GetMaxUploadSizePerBlobBytes() int64 {
	if x != nil && x.MaxUploadSizePerBlobBytes != nil {
		return *x.MaxUploadSizePerBlobBytes
	}
	return 0
}

func (x *CreateUploadURLRequest) GetGsBucketName() string {
	if x != nil && x.GsBucketName != nil {
		return *x.GsBucketName
	}
	return ""
}

func (x *CreateUploadURLRequest) GetUrlExpiryTimeSeconds() int32 {
	if x != nil && x.UrlExpiryTimeSeconds != nil {
		return *x.UrlExpiryTimeSeconds
	}
	return 0
}

type CreateUploadURLResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Url string `protobuf:"bytes,1,opt,name=url,proto3" json:"url,omitempty"`
}

func (x *CreateUploadURLResponse) Reset() {
	*x = CreateUploadURLResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateUploadURLResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateUploadURLResponse) ProtoMessage() {}

func (x *CreateUploadURLResponse) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateUploadURLResponse.ProtoReflect.Descriptor instead.
func (*CreateUploadURLResponse) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{2}
}

func (x *CreateUploadURLResponse) GetUrl() string {
	if x != nil {
		return x.Url
	}
	return ""
}

type DeleteBlobRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlobKey []string `protobuf:"bytes,1,rep,name=blob_key,json=blobKey,proto3" json:"blob_key,omitempty"`
	Token   *string  `protobuf:"bytes,2,opt,name=token,proto3,oneof" json:"token,omitempty"`
}

func (x *DeleteBlobRequest) Reset() {
	*x = DeleteBlobRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteBlobRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteBlobRequest) ProtoMessage() {}

func (x *DeleteBlobRequest) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteBlobRequest.ProtoReflect.Descriptor instead.
func (*DeleteBlobRequest) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{3}
}

func (x *DeleteBlobRequest) GetBlobKey() []string {
	if x != nil {
		return x.BlobKey
	}
	return nil
}

func (x *DeleteBlobRequest) GetToken() string {
	if x != nil && x.Token != nil {
		return *x.Token
	}
	return ""
}

type FetchDataRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlobKey    string `protobuf:"bytes,1,opt,name=blob_key,json=blobKey,proto3" json:"blob_key,omitempty"`
	StartIndex int64  `protobuf:"varint,2,opt,name=start_index,json=startIndex,proto3" json:"start_index,omitempty"`
	EndIndex   int64  `protobuf:"varint,3,opt,name=end_index,json=endIndex,proto3" json:"end_index,omitempty"`
}

func (x *FetchDataRequest) Reset() {
	*x = FetchDataRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FetchDataRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetchDataRequest) ProtoMessage() {}

func (x *FetchDataRequest) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetchDataRequest.ProtoReflect.Descriptor instead.
func (*FetchDataRequest) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{4}
}

func (x *FetchDataRequest) GetBlobKey() string {
	if x != nil {
		return x.BlobKey
	}
	return ""
}

func (x *FetchDataRequest) GetStartIndex() int64 {
	if x != nil {
		return x.StartIndex
	}
	return 0
}

func (x *FetchDataRequest) GetEndIndex() int64 {
	if x != nil {
		return x.EndIndex
	}
	return 0
}

type FetchDataResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte `protobuf:"bytes,1000,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *FetchDataResponse) Reset() {
	*x = FetchDataResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FetchDataResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetchDataResponse) ProtoMessage() {}

func (x *FetchDataResponse) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetchDataResponse.ProtoReflect.Descriptor instead.
func (*FetchDataResponse) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{5}
}

func (x *FetchDataResponse) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type CloneBlobRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlobKey     []byte `protobuf:"bytes,1,opt,name=blob_key,json=blobKey,proto3" json:"blob_key,omitempty"`
	MimeType    []byte `protobuf:"bytes,2,opt,name=mime_type,json=mimeType,proto3" json:"mime_type,omitempty"`
	TargetAppId []byte `protobuf:"bytes,3,opt,name=target_app_id,json=targetAppId,proto3" json:"target_app_id,omitempty"`
}

func (x *CloneBlobRequest) Reset() {
	*x = CloneBlobRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CloneBlobRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloneBlobRequest) ProtoMessage() {}

func (x *CloneBlobRequest) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloneBlobRequest.ProtoReflect.Descriptor instead.
func (*CloneBlobRequest) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{6}
}

func (x *CloneBlobRequest) GetBlobKey() []byte {
	if x != nil {
		return x.BlobKey
	}
	return nil
}

func (x *CloneBlobRequest) GetMimeType() []byte {
	if x != nil {
		return x.MimeType
	}
	return nil
}

func (x *CloneBlobRequest) GetTargetAppId() []byte {
	if x != nil {
		return x.TargetAppId
	}
	return nil
}

type CloneBlobResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlobKey []byte `protobuf:"bytes,1,opt,name=blob_key,json=blobKey,proto3" json:"blob_key,omitempty"`
}

func (x *CloneBlobResponse) Reset() {
	*x = CloneBlobResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CloneBlobResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloneBlobResponse) ProtoMessage() {}

func (x *CloneBlobResponse) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloneBlobResponse.ProtoReflect.Descriptor instead.
func (*CloneBlobResponse) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{7}
}

func (x *CloneBlobResponse) GetBlobKey() []byte {
	if x != nil {
		return x.BlobKey
	}
	return nil
}

type DecodeBlobKeyRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlobKey []string `protobuf:"bytes,1,rep,name=blob_key,json=blobKey,proto3" json:"blob_key,omitempty"`
}

func (x *DecodeBlobKeyRequest) Reset() {
	*x = DecodeBlobKeyRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DecodeBlobKeyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DecodeBlobKeyRequest) ProtoMessage() {}

func (x *DecodeBlobKeyRequest) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DecodeBlobKeyRequest.ProtoReflect.Descriptor instead.
func (*DecodeBlobKeyRequest) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{8}
}

func (x *DecodeBlobKeyRequest) GetBlobKey() []string {
	if x != nil {
		return x.BlobKey
	}
	return nil
}

type DecodeBlobKeyResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Decoded []string `protobuf:"bytes,1,rep,name=decoded,proto3" json:"decoded,omitempty"`
}

func (x *DecodeBlobKeyResponse) Reset() {
	*x = DecodeBlobKeyResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DecodeBlobKeyResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DecodeBlobKeyResponse) ProtoMessage() {}

func (x *DecodeBlobKeyResponse) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DecodeBlobKeyResponse.ProtoReflect.Descriptor instead.
func (*DecodeBlobKeyResponse) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{9}
}

func (x *DecodeBlobKeyResponse) GetDecoded() []string {
	if x != nil {
		return x.Decoded
	}
	return nil
}

type CreateEncodedGoogleStorageKeyRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Filename string `protobuf:"bytes,1,opt,name=filename,proto3" json:"filename,omitempty"`
}

func (x *CreateEncodedGoogleStorageKeyRequest) Reset() {
	*x = CreateEncodedGoogleStorageKeyRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateEncodedGoogleStorageKeyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateEncodedGoogleStorageKeyRequest) ProtoMessage() {}

func (x *CreateEncodedGoogleStorageKeyRequest) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateEncodedGoogleStorageKeyRequest.ProtoReflect.Descriptor instead.
func (*CreateEncodedGoogleStorageKeyRequest) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{10}
}

func (x *CreateEncodedGoogleStorageKeyRequest) GetFilename() string {
	if x != nil {
		return x.Filename
	}
	return ""
}

type CreateEncodedGoogleStorageKeyResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlobKey string `protobuf:"bytes,1,opt,name=blob_key,json=blobKey,proto3" json:"blob_key,omitempty"`
}

func (x *CreateEncodedGoogleStorageKeyResponse) Reset() {
	*x = CreateEncodedGoogleStorageKeyResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobstore_service_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateEncodedGoogleStorageKeyResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateEncodedGoogleStorageKeyResponse) ProtoMessage() {}

func (x *CreateEncodedGoogleStorageKeyResponse) ProtoReflect() protoreflect.Message {
	mi := &file_blobstore_service_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateEncodedGoogleStorageKeyResponse.ProtoReflect.Descriptor instead.
func (*CreateEncodedGoogleStorageKeyResponse) Descriptor() ([]byte, []int) {
	return file_blobstore_service_proto_rawDescGZIP(), []int{11}
}

func (x *CreateEncodedGoogleStorageKeyResponse) GetBlobKey() string {
	if x != nil {
		return x.BlobKey
	}
	return ""
}

var File_blobstore_service_proto protoreflect.FileDescriptor

var file_blobstore_service_proto_rawDesc = []byte{
	0x0a, 0x17, 0x62, 0x6c, 0x6f, 0x62, 0x73, 0x74, 0x6f, 0x72, 0x65, 0x5f, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x09, 0x61, 0x70, 0x70, 0x65, 0x6e,
	0x67, 0x69, 0x6e, 0x65, 0x22, 0xeb, 0x01, 0x0a, 0x15, 0x42, 0x6c, 0x6f, 0x62, 0x73, 0x74, 0x6f,
	0x72, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x22, 0xd1,
	0x01, 0x0a, 0x09, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x06, 0x0a, 0x02,
	0x4f, 0x4b, 0x10, 0x00, 0x12, 0x12, 0x0a, 0x0e, 0x49, 0x4e, 0x54, 0x45, 0x52, 0x4e, 0x41, 0x4c,
	0x5f, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x01, 0x12, 0x10, 0x0a, 0x0c, 0x55, 0x52, 0x4c, 0x5f,
	0x54, 0x4f, 0x4f, 0x5f, 0x4c, 0x4f, 0x4e, 0x47, 0x10, 0x02, 0x12, 0x15, 0x0a, 0x11, 0x50, 0x45,
	0x52, 0x4d, 0x49, 0x53, 0x53, 0x49, 0x4f, 0x4e, 0x5f, 0x44, 0x45, 0x4e, 0x49, 0x45, 0x44, 0x10,
	0x03, 0x12, 0x12, 0x0a, 0x0e, 0x42, 0x4c, 0x4f, 0x42, 0x5f, 0x4e, 0x4f, 0x54, 0x5f, 0x46, 0x4f,
	0x55, 0x4e, 0x44, 0x10, 0x04, 0x12, 0x1b, 0x0a, 0x17, 0x44, 0x41, 0x54, 0x41, 0x5f, 0x49, 0x4e,
	0x44, 0x45, 0x58, 0x5f, 0x4f, 0x55, 0x54, 0x5f, 0x4f, 0x46, 0x5f, 0x52, 0x41, 0x4e, 0x47, 0x45,
	0x10, 0x05, 0x12, 0x1d, 0x0a, 0x19, 0x42, 0x4c, 0x4f, 0x42, 0x5f, 0x46, 0x45, 0x54, 0x43, 0x48,
	0x5f, 0x53, 0x49, 0x5a, 0x45, 0x5f, 0x54, 0x4f, 0x4f, 0x5f, 0x4c, 0x41, 0x52, 0x47, 0x45, 0x10,
	0x06, 0x12, 0x19, 0x0a, 0x15, 0x41, 0x52, 0x47, 0x55, 0x4d, 0x45, 0x4e, 0x54, 0x5f, 0x4f, 0x55,
	0x54, 0x5f, 0x4f, 0x46, 0x5f, 0x52, 0x41, 0x4e, 0x47, 0x45, 0x10, 0x08, 0x12, 0x14, 0x0a, 0x10,
	0x49, 0x4e, 0x56, 0x41, 0x4c, 0x49, 0x44, 0x5f, 0x42, 0x4c, 0x4f, 0x42, 0x5f, 0x4b, 0x45, 0x59,
	0x10, 0x09, 0x22, 0x8e, 0x03, 0x0a, 0x16, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x55, 0x70, 0x6c,
	0x6f, 0x61, 0x64, 0x55, 0x52, 0x4c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x21, 0x0a,
	0x0c, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0b, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x50, 0x61, 0x74, 0x68,
	0x12, 0x36, 0x0a, 0x15, 0x6d, 0x61, 0x78, 0x5f, 0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x5f, 0x73,
	0x69, 0x7a, 0x65, 0x5f, 0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x48,
	0x00, 0x52, 0x12, 0x6d, 0x61, 0x78, 0x55, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x53, 0x69, 0x7a, 0x65,
	0x42, 0x79, 0x74, 0x65, 0x73, 0x88, 0x01, 0x01, 0x12, 0x46, 0x0a, 0x1e, 0x6d, 0x61, 0x78, 0x5f,
	0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x5f, 0x70, 0x65, 0x72, 0x5f,
	0x62, 0x6c, 0x6f, 0x62, 0x5f, 0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03,
	0x48, 0x01, 0x52, 0x19, 0x6d, 0x61, 0x78, 0x55, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x53, 0x69, 0x7a,
	0x65, 0x50, 0x65, 0x72, 0x42, 0x6c, 0x6f, 0x62, 0x42, 0x79, 0x74, 0x65, 0x73, 0x88, 0x01, 0x01,
	0x12, 0x29, 0x0a, 0x0e, 0x67, 0x73, 0x5f, 0x62, 0x75, 0x63, 0x6b, 0x65, 0x74, 0x5f, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x48, 0x02, 0x52, 0x0c, 0x67, 0x73, 0x42, 0x75,
	0x63, 0x6b, 0x65, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x88, 0x01, 0x01, 0x12, 0x3a, 0x0a, 0x17, 0x75,
	0x72, 0x6c, 0x5f, 0x65, 0x78, 0x70, 0x69, 0x72, 0x79, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x73,
	0x65, 0x63, 0x6f, 0x6e, 0x64, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x05, 0x48, 0x03, 0x52, 0x14,
	0x75, 0x72, 0x6c, 0x45, 0x78, 0x70, 0x69, 0x72, 0x79, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x63,
	0x6f, 0x6e, 0x64, 0x73, 0x88, 0x01, 0x01, 0x42, 0x18, 0x0a, 0x16, 0x5f, 0x6d, 0x61, 0x78, 0x5f,
	0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x5f, 0x62, 0x79, 0x74, 0x65,
	0x73, 0x42, 0x21, 0x0a, 0x1f, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x75, 0x70, 0x6c, 0x6f, 0x61, 0x64,
	0x5f, 0x73, 0x69, 0x7a, 0x65, 0x5f, 0x70, 0x65, 0x72, 0x5f, 0x62, 0x6c, 0x6f, 0x62, 0x5f, 0x62,
	0x79, 0x74, 0x65, 0x73, 0x42, 0x11, 0x0a, 0x0f, 0x5f, 0x67, 0x73, 0x5f, 0x62, 0x75, 0x63, 0x6b,
	0x65, 0x74, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x42, 0x1a, 0x0a, 0x18, 0x5f, 0x75, 0x72, 0x6c, 0x5f,
	0x65, 0x78, 0x70, 0x69, 0x72, 0x79, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x73, 0x65, 0x63, 0x6f,
	0x6e, 0x64, 0x73, 0x22, 0x2b, 0x0a, 0x17, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x55, 0x70, 0x6c,
	0x6f, 0x61, 0x64, 0x55, 0x52, 0x4c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x10,
	0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x6c,
	0x22, 0x53, 0x0a, 0x11, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x19, 0x0a, 0x08, 0x62, 0x6c, 0x6f, 0x62, 0x5f, 0x6b, 0x65,
	0x79, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x62, 0x6c, 0x6f, 0x62, 0x4b, 0x65, 0x79,
	0x12, 0x19, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48,
	0x00, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x88, 0x01, 0x01, 0x42, 0x08, 0x0a, 0x06, 0x5f,
	0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0x6b, 0x0a, 0x10, 0x46, 0x65, 0x74, 0x63, 0x68, 0x44, 0x61,
	0x74, 0x61, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x19, 0x0a, 0x08, 0x62, 0x6c, 0x6f,
	0x62, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x62, 0x6c, 0x6f,
	0x62, 0x4b, 0x65, 0x79, 0x12, 0x1f, 0x0a, 0x0b, 0x73, 0x74, 0x61, 0x72, 0x74, 0x5f, 0x69, 0x6e,
	0x64, 0x65, 0x78, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0a, 0x73, 0x74, 0x61, 0x72, 0x74,
	0x49, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x1b, 0x0a, 0x09, 0x65, 0x6e, 0x64, 0x5f, 0x69, 0x6e, 0x64,
	0x65, 0x78, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x65, 0x6e, 0x64, 0x49, 0x6e, 0x64,
	0x65, 0x78, 0x22, 0x2c, 0x0a, 0x11, 0x46, 0x65, 0x74, 0x63, 0x68, 0x44, 0x61, 0x74, 0x61, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x17, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18,
	0xe8, 0x07, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x02, 0x08, 0x01, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61,
	0x22, 0x6e, 0x0a, 0x10, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x19, 0x0a, 0x08, 0x62, 0x6c, 0x6f, 0x62, 0x5f, 0x6b, 0x65, 0x79,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x07, 0x62, 0x6c, 0x6f, 0x62, 0x4b, 0x65, 0x79, 0x12,
	0x1b, 0x0a, 0x09, 0x6d, 0x69, 0x6d, 0x65, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x08, 0x6d, 0x69, 0x6d, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x22, 0x0a, 0x0d,
	0x74, 0x61, 0x72, 0x67, 0x65, 0x74, 0x5f, 0x61, 0x70, 0x70, 0x5f, 0x69, 0x64, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x0b, 0x74, 0x61, 0x72, 0x67, 0x65, 0x74, 0x41, 0x70, 0x70, 0x49, 0x64,
	0x22, 0x2e, 0x0a, 0x11, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x62, 0x6c, 0x6f, 0x62, 0x5f, 0x6b, 0x65,
	0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x07, 0x62, 0x6c, 0x6f, 0x62, 0x4b, 0x65, 0x79,
	0x22, 0x31, 0x0a, 0x14, 0x44, 0x65, 0x63, 0x6f, 0x64, 0x65, 0x42, 0x6c, 0x6f, 0x62, 0x4b, 0x65,
	0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x19, 0x0a, 0x08, 0x62, 0x6c, 0x6f, 0x62,
	0x5f, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x62, 0x6c, 0x6f, 0x62,
	0x4b, 0x65, 0x79, 0x22, 0x31, 0x0a, 0x15, 0x44, 0x65, 0x63, 0x6f, 0x64, 0x65, 0x42, 0x6c, 0x6f,
	0x62, 0x4b, 0x65, 0x79, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x18, 0x0a, 0x07,
	0x64, 0x65, 0x63, 0x6f, 0x64, 0x65, 0x64, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x64,
	0x65, 0x63, 0x6f, 0x64, 0x65, 0x64, 0x22, 0x42, 0x0a, 0x24, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x45, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x64, 0x47, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x53, 0x74, 0x6f,
	0x72, 0x61, 0x67, 0x65, 0x4b, 0x65, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1a,
	0x0a, 0x08, 0x66, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x08, 0x66, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x42, 0x0a, 0x25, 0x43, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x45, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x64, 0x47, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x53, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x4b, 0x65, 0x79, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x62, 0x6c, 0x6f, 0x62, 0x5f, 0x6b, 0x65, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x62, 0x6c, 0x6f, 0x62, 0x4b, 0x65, 0x79, 0x42, 0x30,
	0x5a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x67, 0x6f, 0x6c, 0x61, 0x6e, 0x67, 0x2e,
	0x6f, 0x72, 0x67, 0x2f, 0x61, 0x70, 0x70, 0x65, 0x6e, 0x67, 0x69, 0x6e, 0x65, 0x2f, 0x69, 0x6e,
	0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x62, 0x6c, 0x6f, 0x62, 0x73, 0x74, 0x6f, 0x72, 0x65,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_blobstore_service_proto_rawDescOnce sync.Once
	file_blobstore_service_proto_rawDescData = file_blobstore_service_proto_rawDesc
)

func file_blobstore_service_proto_rawDescGZIP() []byte {
	file_blobstore_service_proto_rawDescOnce.Do(func() {
		file_blobstore_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_blobstore_service_proto_rawDescData)
	})
	return file_blobstore_service_proto_rawDescData
}

var file_blobstore_service_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_blobstore_service_proto_msgTypes = make([]protoimpl.MessageInfo, 12)
var file_blobstore_service_proto_goTypes = []interface{}{
	(BlobstoreServiceError_ErrorCode)(0),          // 0: appengine.BlobstoreServiceError.ErrorCode
	(*BlobstoreServiceError)(nil),                 // 1: appengine.BlobstoreServiceError
	(*CreateUploadURLRequest)(nil),                // 2: appengine.CreateUploadURLRequest
	(*CreateUploadURLResponse)(nil),               // 3: appengine.CreateUploadURLResponse
	(*DeleteBlobRequest)(nil),                     // 4: appengine.DeleteBlobRequest
	(*FetchDataRequest)(nil),                      // 5: appengine.FetchDataRequest
	(*FetchDataResponse)(nil),                     // 6: appengine.FetchDataResponse
	(*CloneBlobRequest)(nil),                      // 7: appengine.CloneBlobRequest
	(*CloneBlobResponse)(nil),                     // 8: appengine.CloneBlobResponse
	(*DecodeBlobKeyRequest)(nil),                  // 9: appengine.DecodeBlobKeyRequest
	(*DecodeBlobKeyResponse)(nil),                 // 10: appengine.DecodeBlobKeyResponse
	(*CreateEncodedGoogleStorageKeyRequest)(nil),  // 11: appengine.CreateEncodedGoogleStorageKeyRequest
	(*CreateEncodedGoogleStorageKeyResponse)(nil), // 12: appengine.CreateEncodedGoogleStorageKeyResponse
}
var file_blobstore_service_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_blobstore_service_proto_init() }
func file_blobstore_service_proto_init() {
	if File_blobstore_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_blobstore_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BlobstoreServiceError); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateUploadURLRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateUploadURLResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteBlobRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FetchDataRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FetchDataResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CloneBlobRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CloneBlobResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DecodeBlobKeyRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DecodeBlobKeyResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateEncodedGoogleStorageKeyRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobstore_service_proto_msgTypes[11].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateEncodedGoogleStorageKeyResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_blobstore_service_proto_msgTypes[1].OneofWrappers = []interface{}{}
	file_blobstore_service_proto_msgTypes[3].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_blobstore_service_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   12,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_blobstore_service_proto_goTypes,
		DependencyIndexes: file_blobstore_service_proto_depIdxs,
		EnumInfos:         file_blobstore_service_proto_enumTypes,
		MessageInfos:      file_blobstore_service_proto_msgTypes,
	}.Build()
	File_blobstore_service_proto = out.File
	file_blobstore_service_proto_rawDesc = nil
	file_blobstore_service_proto_goTypes = nil
	file_blobstore_service_proto_depIdxs = nil
}
