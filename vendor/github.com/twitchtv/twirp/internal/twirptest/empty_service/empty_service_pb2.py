# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: empty_service.proto

from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from google.protobuf import reflection as _reflection
from google.protobuf import symbol_database as _symbol_database
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()




DESCRIPTOR = _descriptor.FileDescriptor(
  name='empty_service.proto',
  package='twirp.internal.twirptest.emptyservice',
  syntax='proto3',
  serialized_options=b'Z\rempty_service',
  serialized_pb=b'\n\x13\x65mpty_service.proto\x12%twirp.internal.twirptest.emptyservice2\x07\n\x05\x45mptyB\x0fZ\rempty_serviceb\x06proto3'
)



_sym_db.RegisterFileDescriptor(DESCRIPTOR)


DESCRIPTOR._options = None

_EMPTY = _descriptor.ServiceDescriptor(
  name='Empty',
  full_name='twirp.internal.twirptest.emptyservice.Empty',
  file=DESCRIPTOR,
  index=0,
  serialized_options=None,
  serialized_start=62,
  serialized_end=69,
  methods=[
])
_sym_db.RegisterServiceDescriptor(_EMPTY)

DESCRIPTOR.services_by_name['Empty'] = _EMPTY

# @@protoc_insertion_point(module_scope)
