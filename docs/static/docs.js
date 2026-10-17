(() => {
  "use strict";

  const menu = document.querySelector("[data-docs-menu]");
  if (menu) {
    const desktop = window.matchMedia("(min-width: 941px)");
    const syncMenu = () => {
      menu.open = desktop.matches;
    };
    syncMenu();
    desktop.addEventListener("change", syncMenu);
  }

  const content = document.querySelector("[data-page-content]");
  const headings = content
    ? [...content.querySelectorAll("h2[id], h3[id]")]
    : [];
  const tocLinks = new Map();
  if (headings.length > 1) {
    document.querySelectorAll("[data-table-of-contents]").forEach((nav) => {
      const list = document.createElement("ol");
      headings.forEach((heading) => {
        const item = document.createElement("li");
        const link = document.createElement("a");
        link.href = `#${heading.id}`;
        link.textContent = heading.textContent;
        item.className = heading.tagName === "H3" ? "toc-nested" : "";
        item.append(link);
        list.append(item);
        const links = tocLinks.get(heading.id) || [];
        links.push(link);
        tocLinks.set(heading.id, links);
      });
      nav.append(list);
    });
    document
      .querySelectorAll("[data-page-toc], [data-inline-toc]")
      .forEach((element) => {
        element.hidden = false;
      });
    const markCurrent = (id) =>
      tocLinks.forEach((links, key) =>
        links.forEach((link) => {
          if (key === id) link.setAttribute("aria-current", "location");
          else link.removeAttribute("aria-current");
        }),
      );
    markCurrent(headings[0].id);
    if ("IntersectionObserver" in window) {
      const observer = new IntersectionObserver(
        (entries) => {
          const visible = entries
            .filter((entry) => entry.isIntersecting)
            .sort(
              (a, b) => a.boundingClientRect.top - b.boundingClientRect.top,
            );
          if (visible.length) markCurrent(visible[0].target.id);
        },
        { rootMargin: "-100px 0px -65% 0px", threshold: 0 },
      );
      headings.forEach((heading) => observer.observe(heading));
    }
  }

  headings.forEach((heading) => {
    const anchor = document.createElement("a");
    anchor.href = `#${heading.id}`;
    anchor.className = "heading-anchor";
    anchor.setAttribute("aria-label", `Link to ${heading.textContent}`);
    anchor.textContent = "#";
    heading.append(anchor);
  });

  if (navigator.clipboard?.writeText) {
    document.querySelectorAll(".prose pre").forEach((pre) => {
      const code = pre.querySelector("code");
      if (!code) return;
      const frame = document.createElement("div");
      const toolbar = document.createElement("div");
      const label = document.createElement("span");
      const button = document.createElement("button");
      frame.className = "code-frame";
      toolbar.className = "code-toolbar";
      label.textContent = "Code example";
      button.className = "copy-button";
      button.type = "button";
      button.textContent = "Copy";
      button.setAttribute("aria-label", "Copy code to clipboard");
      button.setAttribute("aria-live", "polite");
      button.addEventListener("click", async () => {
        try {
          await navigator.clipboard.writeText(code.textContent);
          button.textContent = "Copied";
          button.setAttribute("aria-label", "Code copied to clipboard");
        } catch {
          button.textContent = "Select to copy";
          button.setAttribute(
            "aria-label",
            "Copy unavailable; select the code manually",
          );
        }
        window.setTimeout(() => {
          button.textContent = "Copy";
          button.setAttribute("aria-label", "Copy code to clipboard");
        }, 1800);
      });
      toolbar.append(label, button);
      pre.before(frame);
      frame.append(toolbar, pre);
    });
  }

  document.querySelectorAll(".prose table").forEach((table) => {
    const wrapper = document.createElement("div");
    wrapper.className = "table-wrap";
    wrapper.setAttribute("role", "region");
    wrapper.setAttribute("aria-label", "Reference table");
    table.before(wrapper);
    wrapper.append(table);
    table.querySelectorAll("th").forEach((cell) => {
      cell.scope = "col";
    });
    const syncFocus = () => {
      if (wrapper.scrollWidth > wrapper.clientWidth) wrapper.tabIndex = 0;
      else wrapper.removeAttribute("tabindex");
    };
    syncFocus();
    if ("ResizeObserver" in window)
      new ResizeObserver(syncFocus).observe(wrapper);
  });

  const form = document.querySelector("[data-header-search]");
  const input = document.querySelector("[data-header-search-input]");
  const panel = document.querySelector("[data-header-search-panel]");
  const results = document.querySelector("[data-header-search-results]");
  const status = document.querySelector("[data-header-search-status]");
  if (!form || !input || !panel || !results || !status) return;
  const basePath = form.getAttribute("data-base-path") || "";
  const docs = window.searchIndex?.documentStore?.docs || {};
  const entries = Object.entries(docs).map(([url, doc]) => ({
    url,
    title: doc.title || url,
    description: doc.description || "",
    body: (doc.body || "").replace(/\s+/g, " "),
  }));
  let matches = [];
  const close = () => {
    panel.hidden = true;
    input.setAttribute("aria-expanded", "false");
    results.replaceChildren();
    status.textContent = "";
    matches = [];
  };
  const render = () => {
    const terms = input.value.toLowerCase().trim().split(/\s+/).filter(Boolean);
    if (!terms.length) {
      close();
      return;
    }
    matches = entries
      .map((entry) => {
        const title = entry.title.toLowerCase();
        const description = entry.description.toLowerCase();
        const body = entry.body.toLowerCase();
        const score = terms.every((term) =>
          `${title} ${description} ${body}`.includes(term),
        )
          ? terms.reduce(
              (total, term) =>
                total +
                (title.includes(term) ? 8 : 0) +
                (description.includes(term) ? 4 : 0) +
                (body.includes(term) ? 1 : 0),
              0,
            )
          : 0;
        return { entry, score };
      })
      .filter((match) => match.score > 0)
      .sort(
        (a, b) =>
          b.score - a.score || a.entry.title.localeCompare(b.entry.title),
      )
      .slice(0, 6);
    results.replaceChildren();
    status.textContent = matches.length
      ? `${matches.length} ${matches.length === 1 ? "result" : "results"}`
      : "No results. Try a command, setting, or topic.";
    matches.forEach(({ entry }) => {
      const item = document.createElement("li");
      const link = document.createElement("a");
      const summary = document.createElement("p");
      link.href = basePath + entry.url;
      link.textContent = entry.title;
      summary.textContent = entry.description || entry.body.slice(0, 150);
      item.append(link, summary);
      results.append(item);
    });
    panel.hidden = false;
    input.setAttribute("aria-expanded", "true");
  };
  input.addEventListener("input", render);
  input.addEventListener("focus", render);
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    if (matches.length) window.location.href = basePath + matches[0].entry.url;
  });
  document.addEventListener("click", (event) => {
    if (!form.contains(event.target)) close();
  });
  document.addEventListener("keydown", (event) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
      event.preventDefault();
      input.focus();
    }
    if (event.key === "Escape") {
      close();
      input.blur();
    }
  });
})();
