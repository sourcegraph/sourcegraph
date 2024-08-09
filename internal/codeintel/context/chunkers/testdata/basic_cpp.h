/*
 *
 * Copyright 2015-2016 gRPC authors.
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
 */

#ifndef GRPC_GRPC_H
#define GRPC_GRPC_H

#include <grpc/support/port_platform.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/*! \mainpage GRPC Core
 *
 * The GRPC Core library is a low-level library designed to be wrapped by higher
 * level libraries. The top-level API is provided in grpc.h. Security related
 * functionality lives in grpc_security.h.
 */

GRPCAPI void grpc_metadata_array_init(grpc_metadata_array* array);

/** Returns the completion queue factory based on the attributes. MAY return a
    NULL if no factory can be found */
GRPCAPI const grpc_completion_queue_factory*
grpc_completion_queue_factory_lookup(
    const grpc_completion_queue_attributes* attributes);

/** Maximum number of outstanding grpc_completion_queue_pluck executions per
    completion queue */
#define GRPC_MAX_COMPLETION_QUEUE_PLUCKERS 6

typedef struct grpc_channel_credentials grpc_channel_credentials;

/** How to handle payloads for a registered method */
typedef enum {
  /** Don't try to read the payload */
  GRPC_SRM_PAYLOAD_NONE,
  /** Read the initial payload as a byte buffer */
  GRPC_SRM_PAYLOAD_READ_INITIAL_BYTE_BUFFER
} grpc_server_register_method_payload_handling;

// More members might be added in later, so users should take care to memset
// this to 0 before using it.
typedef struct {
  grpc_status_code code;
  const char* error_message;
} grpc_serving_status_update;

#ifdef __cplusplus
}
#endif

#endif /* GRPC_GRPC_H */

namespace N
{
    class my_class
    {
    public:
        void do_something();
    };

}
