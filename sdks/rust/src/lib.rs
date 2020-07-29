// Copyright 2018 Google LLC All Rights Reserved.
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

//! the Rust game server SDK
#[macro_use]
extern crate error_chain;
extern crate futures;
extern crate grpcio;
extern crate grpcio_proto;
extern crate protobuf;

pub mod alpha;
pub mod errors;
mod grpc;
mod sdk;
pub mod types;

pub use sdk::Sdk;
