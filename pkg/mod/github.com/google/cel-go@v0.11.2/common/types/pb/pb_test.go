// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pb

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"

	proto3pb "github.com/google/cel-go/test/proto3pb"
	descpb "google.golang.org/protobuf/types/descriptorpb"
)

func TestDbCopy(t *testing.T) {
	clone := DefaultDb.Copy()
	if !reflect.DeepEqual(clone, DefaultDb) {
		t.Error("db.Copy() did not result in eqivalent objects.")
	}
	_, err := clone.RegisterMessage(&proto3pb.TestAllTypes{})
	if err != nil {
		t.Fatalf("db.RegisterMessage() failed: %v", err)
	}
	if reflect.DeepEqual(clone, DefaultDb) {
		t.Error("db.RegisterMessage() altered the default db as well")
	}
	clone2 := clone.Copy()
	if !reflect.DeepEqual(clone, clone2) {
		t.Error("db.Copy() did not result in eqivalent objects.")
	}
}

func BenchmarkDbCopy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewDb().Copy()
	}
}

func TestProtoReflectRoundTrip(t *testing.T) {
	msg := &proto3pb.TestAllTypes{SingleBool: true}
	fdMap := CollectFileDescriptorSet(msg)
	files := []*descpb.FileDescriptorProto{}
	for _, fd := range fdMap {
		files = append(files, protodesc.ToFileDescriptorProto(fd))
	}
	// Round-tripping from a protoreflect.FileDescriptor to a FileDescriptorSet and back
	// will result in completely independent copies of the protoreflect.FileDescriptor
	// whose values are incompatible with each other.
	//
	// This test showcases what happens when the protoregistry.GlobalFiles values are
	// used when a given protoreflect.FileDescriptor is linked into the binary.
	fds := &descpb.FileDescriptorSet{File: files}
	pbReg, err := protodesc.NewFiles(fds)
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}
	pbdb := NewDb()
	pbReg.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		_, err := pbdb.RegisterDescriptor(fd)
		if err != nil {
			t.Fatalf("pbdb.RegisterDecriptor(%v) failed: %v", fd, err)
		}
		return true
	})
	msgType, found := pbdb.DescribeType("google.expr.proto3.test.TestAllTypes")
	if !found {
		t.Fatal("pbdb.DescribeType(google.expr.proto3.test.TestAllTypes) failed")
	}
	boolField, found := msgType.FieldByName("single_bool")
	if !found {
		t.Fatal("msgType.FieldByName(single_bool) failed")
	}
	val, err := boolField.GetFrom(msg)
	if err != nil {
		t.Errorf("boolField.GetFrom(msg) failed: %v", err)
	}
	if val != true {
		t.Errorf("got TestAllTypes.single_bool %v, wanted true", val)
	}
}
