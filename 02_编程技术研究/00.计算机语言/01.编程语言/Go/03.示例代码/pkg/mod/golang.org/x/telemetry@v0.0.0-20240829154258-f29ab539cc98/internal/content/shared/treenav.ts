/**
 * @license
 * Copyright 2024 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

/**
 * A treeNavController adds dynamic expansion and selection of index list
 * elements based on scroll position.
 *
 * Use it as follows:
 *  - Add the .js-Tree class to a parent element of your index and content.
 *  - Add the .js-Tree-item class to <li> elements of your index.
 *  - Add the .js-Tree-heading class to <hN> heading elements of your content.
 *
 * Then, when you scroll content, the 'aria-selected' and 'aria-expanded'
 * attributes of your tree items will be set according to the current content
 * scroll position. The included treenav.css implements styling to expand and
 * highlight index elements according to these attributes.
 */
export function treeNavController(el: HTMLElement) {
  const headings = el.querySelectorAll<HTMLHeadingElement>(".js-Tree-heading");
  const callback = () => {
    // Collect heading elements above the scroll position.
    let above: HTMLHeadingElement[] = [];
    for (const h of headings) {
      const rect = h.getBoundingClientRect();
      if (rect.height && rect.top < 80) {
        above.unshift(h);
      }
    }
    // Highlight the first heading even if we're not yet scrolled below it.
    if (above.length == 0 && headings[0] instanceof HTMLHeadingElement) {
      above = [headings[0]];
    }
    // Collect the set of heading levels we're immediately below, at most one
    // per heading level, by decresing level.
    // e.g. [<h3 element>, <h2 element>, <h1 element>]
    let threshold = Infinity;
    const active: HTMLHeadingElement[] = [];
    for (const h of above) {
      const level = Number(h.tagName[1]);
      if (level < threshold) {
        threshold = level;
        active.push(h);
      }
    }
    // Update aria-selected and aria-expanded for all items, per the current
    // position.
    const navItems = el.querySelectorAll<HTMLElement>(".js-Tree-item");
    for (const item of navItems) {
      const headingId = item.dataset["headingId"];
      let selected = false,
        expanded = false;
      for (const h of active) {
        if (h.id === headingId) {
          if (h === active[0]) {
            selected = true;
          } else {
            expanded = true;
          }
          break;
        }
      }
      item.setAttribute("aria-selected", selected ? "true" : "false");
      item.setAttribute("aria-expanded", expanded ? "true" : "false");
    }
  };

  // Update on changes to viewport intersection, defensively debouncing to
  // guard against performance issues.
  const observer = new IntersectionObserver(debounce(callback, 20));
  for (const h of headings) {
    observer.observe(h);
  }
}

export function debounce<T extends (...args: unknown[]) => unknown>(
  callback: T,
  wait: number
) {
  let timeout: number;
  return (...args: unknown[]) => {
    clearTimeout(timeout);
    timeout = setTimeout(() => callback(...args), wait);
  };
}
