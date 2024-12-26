/**
 * @license
 * Copyright 2023 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import { ToolTipController } from "./_tooltip";

for (const el of document.querySelectorAll<HTMLDetailsElement>(".js-tooltip")) {
  new ToolTipController(el);
}
