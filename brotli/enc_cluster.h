/* Copyright 2013 Google Inc. All Rights Reserved.

   Distributed under MIT license.
   See file LICENSE for detail or copy at https://opensource.org/licenses/MIT
*/

/* Functions for clustering similar histograms together. */

#ifndef BROTLI_ENC_CLUSTER_H_
#define BROTLI_ENC_CLUSTER_H_

#include "brotli_types.h"

#include "common_platform.h"
#include "enc_histogram.h"
#include "enc_memory.h"

#if defined(__cplusplus) || defined(c_plusplus)
extern "C" {
#endif

typedef struct HistogramPair {
  uint32_t idx1;
  uint32_t idx2;
  double cost_combo;
  double cost_diff;
} HistogramPair;

#define CODE(X) /* Declaration */;

#define FN(X) X ## Literal
#include "enc_cluster_inc.h"  /* NOLINT(build/include) */
#undef FN

#define FN(X) X ## Command
#include "enc_cluster_inc.h"  /* NOLINT(build/include) */
#undef FN

#define FN(X) X ## Distance
#include "enc_cluster_inc.h"  /* NOLINT(build/include) */
#undef FN

#undef CODE

#if defined(__cplusplus) || defined(c_plusplus)
}  /* extern "C" */
#endif

#endif  /* BROTLI_ENC_CLUSTER_H_ */
