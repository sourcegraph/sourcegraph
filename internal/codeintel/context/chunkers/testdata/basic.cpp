//
// Copyright 2018 gRPC authors.
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
//
//

#include <cstdio>
#include <atomic>
#include <functional>
#include <memory>
#include <utility>

#include <grpc/support/port_platform.h>
#include "absl/status/status.h"

#include <grpc/event_engine/event_engine.h>
#include <grpc/grpc.h>
#include <grpc/support/log.h>
#include <grpc/support/sync.h>
#include <grpc/support/time.h>
#include <grpcpp/alarm.h>
#include <grpcpp/completion_queue.h>
#include <grpcpp/impl/completion_queue_tag.h>

#include "src/core/lib/event_engine/default_event_engine.h"
#include "src/core/lib/gprpp/time.h"
#include "src/core/lib/iomgr/error.h"
#include "src/core/lib/iomgr/exec_ctx.h"
#include "src/core/lib/surface/completion_queue.h"

namespace grpc {

namespace internal {

namespace {
using grpc_event_engine::experimental::EventEngine;
}  // namespace

class AlarmImpl : public grpc::internal::CompletionQueueTag {
 public:
  AlarmImpl()
      : event_engine_(grpc_event_engine::experimental::GetDefaultEventEngine()),
        cq_(nullptr),
        tag_(nullptr) {
    gpr_ref_init(&refs_, 1);
  }
  ~AlarmImpl() override {}
  bool FinalizeResult(void** tag, bool* /*status*/) override {
    *tag = tag_;
    Unref();
    return true;
  }
  void Set(grpc::CompletionQueue* cq, gpr_timespec deadline, void* tag) {
    grpc_core::ExecCtx exec_ctx;
    GRPC_CQ_INTERNAL_REF(cq->cq(), "alarm");
    cq_ = cq->cq();
    tag_ = tag;
    GPR_ASSERT(grpc_cq_begin_op(cq_, this));
    Ref();
    GPR_ASSERT(cq_armed_.exchange(true) == false);
    GPR_ASSERT(!callback_armed_.load());
    cq_timer_handle_ = event_engine_->RunAfter(
        grpc_core::Timestamp::FromTimespecRoundUp(deadline) -
            grpc_core::ExecCtx::Get()->Now(),
        [this] { OnCQAlarm(absl::OkStatus()); });
  }
  void Set(gpr_timespec deadline, std::function<void(bool)> f) {
    grpc_core::ExecCtx exec_ctx;
    // Don't use any CQ at all. Instead just use the timer to fire the function
    callback_ = std::move(f);
    Ref();
    GPR_ASSERT(callback_armed_.exchange(true) == false);
    GPR_ASSERT(!cq_armed_.load());
    callback_timer_handle_ = event_engine_->RunAfter(
        grpc_core::Timestamp::FromTimespecRoundUp(deadline) -
            grpc_core::ExecCtx::Get()->Now(),
        [this] { OnCallbackAlarm(true); });
  }

 private:
  void OnCQAlarm(grpc_error_handle error) {
    cq_armed_.store(false);
    grpc_core::ApplicationCallbackExecCtx callback_exec_ctx;
    grpc_core::ExecCtx exec_ctx;
    // Preserve the cq and reset the cq_ so that the alarm
    // can be reset when the alarm tag is delivered.
    grpc_completion_queue* cq = cq_;
    cq_ = nullptr;
    grpc_cq_end_op(
        cq, this, error,
        [](void* /*arg*/, grpc_cq_completion* /*completion*/) {}, nullptr,
        &completion_);
    GRPC_CQ_INTERNAL_UNREF(cq, "alarm");
  }

  std::shared_ptr<grpc_event_engine::experimental::EventEngine> event_engine_;
  std::function<void(bool)> callback_;
};
}  // namespace internal

Alarm::Alarm() : alarm_(new internal::AlarmImpl()) {}

void Alarm::SetInternal(grpc::CompletionQueue* cq, gpr_timespec deadline,
                        void* tag) {
  // Note that we know that alarm_ is actually an internal::AlarmImpl
  // but we declared it as the base pointer to avoid a forward declaration
  // or exposing core data structures in the C++ public headers.
  // Thus it is safe to use a static_cast to the subclass here, and the
  // C++ style guide allows us to do so in this case
  static_cast<internal::AlarmImpl*>(alarm_)->Set(cq, deadline, tag);
}

Alarm::~Alarm() {
  if (alarm_ != nullptr) {
    static_cast<internal::AlarmImpl*>(alarm_)->Destroy();
  }
}

void Alarm::Cancel() { static_cast<internal::AlarmImpl*>(alarm_)->Cancel(); }
}  // namespace grpc

//----------------------------------------------------------------------------
//
//    FUNCTION: mulnum
//
//    ARGUMENTS: pointer to a number a second number, and the
//               radix.
//
//    RETURN: None, changes first pointer.
//
//    DESCRIPTION: Does the number equivalent of *pa *= b.
//    Assumes radix is the radix of both numbers.  This algorithm is the
//    same one you learned in grade school.
//
//----------------------------------------------------------------------------

void _mulnum(PNUMBER* pa, PNUMBER b, uint32_t radix);
