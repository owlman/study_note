/**
 * @license
 * Copyright 2023 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import * as Plot from "@observablehq/plot";

import { debounce } from "../shared/treenav";

declare global {
  interface Page {
    Charts: ChartData;
  }

  interface ChartData {
    Programs: Program[];
    DateRange: [string, string];
    UploadDay: Plot.TimeIntervalName;
  }

  interface Program {
    ID: string;
    Name: string;
    Counters: Counter[];
    Active: boolean;
  }

  interface Counter {
    ID: string;
    Name: string;
    Data: Datum[];
  }

  interface Datum {
    [key: string]: any;
    Week: string;
    Program: string;
    Version: string;
    GOARCH: string;
    GOOS: string;
    GoVersion: string;
    Key: string;
    Value: number;
  }
  const Page: Page;
}

window.onload = function () {
  drawCharts();
  configSelector();
  breadcrumbController();
  sectionController();
};

// sectionController adds event listeners to the section headers
// to toggle them open and closed.
function sectionController() {
  const html = document.querySelector("html")!;
  for (const e of document.querySelectorAll("h2")) {
    e.addEventListener("click", function () {
      let closed = localStorage.getItem("closed-sections")?.split(",");
      if (closed?.includes(this.id)) {
        closed = closed.filter((v) => v !== this.id);
        const str = closed.join(",");
        localStorage.setItem("closed-sections", str);
        html.setAttribute("data-closed-sections", str);
      } else {
        closed = [this.id].concat(closed ?? []);
        const str = closed.join(",");
        localStorage.setItem("closed-sections", str);
        html.setAttribute("data-closed-sections", str);
      }
    });
  }
}

// drawCharts draws the charts using @observable/plot. It is called when
// the page is first rendered and when a facet is selected.
function drawCharts() {
  for (const program of Page.Charts.Programs ?? []) {
    for (const counter of program.Counters ?? []) {
      const rectYOpts: Plot.BinXInputs<Plot.RectYOptions> = {
        tip: true,
        x: (d: Datum) => new Date(d.Week),
        y: (d: Datum) => d.Value,
        interval: Page.Charts.UploadDay,
        fill: (d: Datum) => {
          const n = Number(d.Key);
          return isNaN(n) ? d.Key : n;
        },
      };

      const chart = Plot.plot({
        nice: true,
        x: {
          type: "utc",
          domain: Page.Charts.DateRange.map((d) => new Date(d)),
          label: "Week",
        },
        y: {
          label: "Value",
        },
        color: {
          type: "ordinal",
          legend: true,
          scheme: "Spectral",
          reverse: true,
          label: "Counter",
        },
        height: 256,
        style: "overflow:visible;width:100%;background:transparent",
        marks: [
          Plot.rectY(counter.Data, Plot.binX({ y: "sum" }, rectYOpts)),
          Plot.ruleY([0]),
        ],
      });
      document
        .querySelector(`[data-chart-id="${counter.ID}"]`)
        ?.replaceChildren(chart);
    }
  }
}

// configSelector adds an event listener that reloads the page when a config
// version is selected.
function configSelector() {
  const el = document.querySelector<HTMLButtonElement>(".js-selectConfig");
  el?.addEventListener("change", () => {
    const params = new URLSearchParams(location.search);
    params.set(el.name, el.value);
    history.replaceState(null, "", "?" + params.toString());
    location.reload();
  });
}

// breadcrumbController updates the navigation header as the user scrolls
// that page displaying information about the content currently in the
// viewport.
function breadcrumbController() {
  const headings =
    document.querySelectorAll<HTMLHeadingElement>("h1, h2, h3, h4");
  const callback = debounce(() => {
    let above: HTMLHeadingElement[] = [];
    for (const h of headings) {
      const rect = h.getBoundingClientRect();
      if (rect.height && rect.top < 80) {
        above.unshift(h);
      }
    }
    if (above.length < 2) {
      above = [];
    }
    let threshold = Infinity;
    const els: HTMLHeadingElement[] = [];
    for (const h of above) {
      const level = Number(h.tagName[1]);
      if (level < threshold) {
        threshold = level;
        els.unshift(h);
      }
    }
    const breadcrumb = document.querySelector(".js-breadcrumb ol");
    const items = [];
    for (const h of els) {
      breadcrumb?.replaceChildren;
      const li = document.createElement("li");
      const a = document.createElement("a");
      a.href = `#${h.id}`;
      a.innerText = h.getAttribute("data-label") ?? h.innerText;
      li.appendChild(a);
      items.push(li);
    }
    breadcrumb?.replaceChildren(...items);
  }, 100);

  const observer = new IntersectionObserver(callback);
  for (const h of headings) {
    observer.observe(h);
  }
}

export {};
