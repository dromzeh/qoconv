// qoconv front end. Talks to Go through Wails bindings (window.go.gui.App)
// and the Wails runtime (window.runtime); see internal/gui/app.go.

const api = () => window.go.gui.App;
const $ = (id) => document.getElementById(id);

const state = {
  skin: null,           // SkinInfo
  preview: null,        // Preview
  mode: null,           // keymode shown in the preview and Layout rows
  modes: [],            // keymodes to convert
  overrides: new Map(), // `${mode}:${key}` -> value
  playfield: null,      // last drawn Playfield, redrawn on resize
  busy: false,
  seq: 0,               // latest Convert call; older results are dropped
};

const logos = { "osu!": "logos/osu.svg", Quaver: "logos/quaver.svg" };
// Marks shown in white on the convert button.
const buttonLogos = { "osu!": "logos/osu-white.svg", Quaver: "logos/quaver.svg" };

// Shown, disabled, before a skin is loaded.
const placeholderRows = ["Hit position", "Column start", "Column width", "Column spacing", "Combo position", "Judgement position", "Light position"];

function h(tag, props = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (v == null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? "" : v);
  }
  el.append(...children.flat().filter((c) => c != null && c !== false));
  return el;
}

function svg(markup, viewBox = "0 0 10 10") {
  const el = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  el.setAttribute("viewBox", viewBox);
  el.setAttribute("aria-hidden", "true");
  el.innerHTML = markup;
  return el;
}

// ---------- status line ----------

// The status line shares its spot with the "Made with" credit, which comes
// back whenever there's nothing to report.
function status(text, { error = false, links = [] } = {}) {
  const s = $("status");
  s.classList.toggle("error", error);
  s.replaceChildren(text, ...links.map(([label, fn]) => h("a", { onclick: fn }, label)));
  s.hidden = !text;
  $("made").hidden = !!text;
}

const errText = (e) => String(e?.message ?? e ?? "Something went wrong").replace(/^Error:\s*/, "");

function setBusy(on) {
  state.busy = on;
  $("convert").disabled = on || !state.preview;
}

// ---------- loading ----------

async function load(path) {
  if (!path || state.busy) return;
  closePicker();
  status("Reading skin…");
  try {
    const skin = await api().Load(path);
    Object.assign(state, { skin, preview: null, modes: [...skin.modes], mode: skin.modes[0] });
    state.overrides.clear();
    renderSkin();
    await convert();
  } catch (e) {
    status(errText(e), { error: true });
  }
}

async function browse(folder) {
  closePicker();
  try {
    const path = await (folder ? api().PickSkinFolder() : api().PickSkin());
    if (path) await load(path);
  } catch (e) {
    status(errText(e), { error: true });
  }
}

// ---------- skin picker ----------

const picker = { open: false, skins: [], shown: [], active: 0 };

async function openPicker() {
  if (picker.open) return closePicker();
  picker.open = true;
  $("picker").hidden = false;
  $("banner").setAttribute("aria-expanded", "true");
  const search = $("picker-search");
  search.value = "";
  search.focus();
  renderPicker();
  try {
    picker.skins = await api().InstalledSkins();
  } catch {
    picker.skins = [];
  }
  if (picker.open) renderPicker();
}

function closePicker() {
  if (!picker.open) return;
  picker.open = false;
  $("picker").hidden = true;
  $("banner").setAttribute("aria-expanded", "false");
}

function renderPicker() {
  const q = $("picker-search").value.trim().toLowerCase();
  picker.shown = picker.skins.filter((s) => s.name.toLowerCase().includes(q));
  picker.active = Math.min(picker.active, Math.max(picker.shown.length - 1, 0));

  const items = [];
  for (const game of ["osu!", "Quaver"]) {
    const group = picker.shown.filter((s) => s.game === game);
    if (!group.length) continue;
    items.push(h("li", { class: "picker-group", role: "presentation" },
      h("img", { src: logos[game], alt: "" }), `${game} skins`));
    for (const s of group) {
      const i = picker.shown.indexOf(s);
      items.push(h("li", {
        class: s.path === state.skin?.path ? "picker-item current" : "picker-item",
        role: "option", "aria-selected": String(i === picker.active), title: s.path, "data-i": i,
        onmousemove: () => setActive(i),
        onclick: () => load(s.path),
      }, s.name));
    }
  }
  if (!items.length) {
    items.push(h("li", { class: "picker-empty" }, picker.skins.length
      ? "No skins match."
      : "No osu! or Quaver skins found. Browse for one below, or drop it on the window."));
  }
  $("picker-list").replaceChildren(...items);
}

function setActive(i) {
  if (i === picker.active) return;
  picker.active = i;
  for (const el of $("picker-list").querySelectorAll(".picker-item")) {
    el.setAttribute("aria-selected", String(Number(el.dataset.i) === i));
  }
}

function pickerKeys(e) {
  const n = picker.shown.length;
  if (e.key === "ArrowDown" || e.key === "ArrowUp") {
    e.preventDefault();
    if (!n) return;
    setActive((picker.active + (e.key === "ArrowDown" ? 1 : n - 1)) % n);
    $("picker-list").querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
  } else if (e.key === "Enter") {
    e.preventDefault();
    const s = picker.shown[picker.active];
    if (s) load(s.path);
  } else if (e.key === "Escape") {
    closePicker();
    $("banner").focus();
  }
}

// ---------- converting (in memory) ----------

async function convert() {
  const seq = ++state.seq;
  setBusy(true);
  status("Converting…");
  try {
    const preview = await api().Convert({
      modes: state.modes,
      grades: $("in-grades").checked,
      hitsounds: $("in-hitsounds").checked,
      healthCW: $("in-healthcw").checked,
      overrides: [...state.overrides].map(([k, value]) => {
        const [mode, key] = k.split(":");
        return { mode: Number(mode), key, value };
      }),
    });
    if (seq !== state.seq) return;
    state.preview = preview;
    if (!preview.modes.some((m) => m.mode === state.mode)) state.mode = preview.modes[0]?.mode;
    renderPreview();
    status("");
  } catch (e) {
    if (seq === state.seq) status(errText(e), { error: true });
  } finally {
    if (seq === state.seq) setBusy(false);
  }
}

// ---------- rendering ----------

function renderSkin() {
  const s = state.skin;
  const root = document.documentElement;
  root.dataset.to = s.to;

  $("banner").classList.add("loaded");
  $("banner-art").style.backgroundImage = s.banner ? `url("${s.banner}")` : "";
  $("banner-empty").hidden = true;
  $("banner-skin").hidden = false;
  $("change").hidden = false;
  $("skin-name").textContent = s.name;
  $("skin-author").textContent = s.author;
  $("direction").hidden = false;
  $("direction").replaceChildren(
    h("img", { src: logos[s.from], alt: s.from }),
    svg('<path d="M1 5h8m0 0L6 2m3 3L6 8"/>'),
    h("img", { src: logos[s.to], alt: s.to }));
  $("direction").title = `${s.from} skin, converting to ${s.to}`;

  $("in-name").value = s.name;
  $("in-author").value = s.author;
  $("in-output").value = s.output;
  $("archive-label").textContent = `Make ${s.ext}`;
  $("install-label").textContent = `Install in ${s.to}`;
  $("convert-label").textContent = `Convert to ${s.to}`;
  $("convert-logo").src = buttonLogos[s.to];
  $("convert-logo").hidden = false;
  $("mode-checks").replaceChildren(...s.modes.map((m) => {
    const box = h("input", { type: "checkbox", checked: state.modes.includes(m) });
    box.addEventListener("change", () => toggleMode(m, box));
    return h("label", {}, box, ` ${m}K`);
  }));
  syncInstall();
}

function toggleMode(m, box) {
  if (!box.checked && state.modes.length === 1) {
    box.checked = true;
    return status("Keep at least one keymode.", { error: true });
  }
  state.modes = box.checked ? [...state.modes, m].sort((a, b) => a - b) : state.modes.filter((x) => x !== m);
  convert();
}

function syncInstall() {
  // Installing works by opening the archive, so it needs one.
  const archive = $("in-archive").checked;
  $("in-install").disabled = !archive;
  if (!archive) $("in-install").checked = false;
}

function renderPreview() {
  const p = state.preview;
  $("mode-tabs").replaceChildren(...p.modes.map((m) => h("button", {
    type: "button", role: "tab", "aria-selected": String(m.mode === state.mode),
    onclick: () => { state.mode = m.mode; renderPreview(); },
  }, `${m.mode}K`)));
  $("mode-tabs").hidden = p.modes.length < 2;

  const view = p.modes.find((m) => m.mode === state.mode);
  if (view) {
    renderPlayfield(view.playfield);
    renderRows(view);
  }
  syncResetAll();
  renderNotes(p.report);
}

const changedIn = (mode) => [...state.overrides.keys()].filter((k) => k.startsWith(`${mode}:`));

function syncResetAll() {
  $("reset-all").hidden = changedIn(state.mode).length === 0 || !$("pane-output").hidden;
}

function renderRows(view) {
  const rows = $("pane-layout");
  rows.classList.remove("empty");
  rows.replaceChildren(...view.fields.map((f) => {
    const key = `${view.mode}:${f.key}`;
    const id = `f-${f.key}`;
    const value = state.overrides.get(key) ?? f.value;
    // The slider covers the usual range, widened to reach the skin's own value.
    const span = f.max - f.min;
    const lo = Math.floor(Math.min(f.min, Number(f.value) - span * 0.1));
    const hi = Math.ceil(Math.max(f.max, Number(f.value) + span * 0.1));
    const input = h("input", { type: "number", id, step: "1", value });
    const slider = h("input", { type: "range", min: lo, max: hi, step: "1", tabindex: "-1", "aria-hidden": "true" });
    const sync = () => {
      const n = Math.min(Math.max(Number(input.value), lo), hi);
      slider.value = String(n);
      slider.style.setProperty("--pct", `${((n - lo) / (hi - lo)) * 100}%`);
    };
    sync();
    const reset = h("button", {
      type: "button", class: "reset", title: "Reset to the converted value", "aria-label": `Reset ${f.label}`,
      onclick: () => resetFields([key]),
    }, svg('<path d="M2.5 6.5a4 4 0 1 0 1-3.6M2.5 1.5v2.4h2.4"/>', "0 0 12 12"));
    const row = h("div", { class: state.overrides.has(key) ? "row changed" : "row", title: `${f.writes || f.key}: ${f.hint}${f.per > 1 ? ` (all ${f.per})` : ""}` },
      h("label", { for: id }, f.label), input, slider, reset);
    const commit = debounce(() => tune(view.mode, f.key, input, row), 100);
    input.addEventListener("input", () => { sync(); commit(); });
    slider.addEventListener("input", () => { input.value = slider.value; sync(); commit(); });
    slider.addEventListener("dblclick", () => resetFields([key]));
    return row;
  }));
}

function renderEmptyRows() {
  const rows = $("pane-layout");
  rows.classList.add("empty");
  rows.replaceChildren(...placeholderRows.map((label) => h("div", { class: "row" },
    h("label", {}, label),
    h("input", { type: "number", disabled: true, placeholder: "–" }),
    h("input", { type: "range", disabled: true, tabindex: "-1", "aria-hidden": "true" }),
    h("span"))));
}

async function tune(mode, key, input, row) {
  const value = input.value.trim();
  if (value === "" || Number.isNaN(Number(value))) return input.setAttribute("aria-invalid", "true");
  input.removeAttribute("aria-invalid");
  try {
    const view = await api().Tune(mode, key, value);
    state.overrides.set(`${mode}:${key}`, value);
    row.classList.add("changed");
    const i = state.preview.modes.findIndex((m) => m.mode === mode);
    if (i >= 0) state.preview.modes[i] = view;
    if (mode === state.mode) renderPlayfield(view.playfield);
    syncResetAll();
  } catch (e) {
    input.setAttribute("aria-invalid", "true");
    status(errText(e), { error: true });
  }
}

// Dropping overrides and converting again restores the computed values.
function resetFields(keys) {
  if (!keys.length) return;
  for (const k of keys) state.overrides.delete(k);
  convert();
}

function renderNotes(r) {
  const game = state.skin.to;
  const lines = [];
  if (r.suppressed.length) lines.push(`Hidden ${game} defaults: ${r.suppressed.join(", ")}.`);
  if (r.dropped.length) lines.push(`Skipped: ${r.dropped.join(", ")}.`);
  if (r.missing.length) lines.push(`${r.missing.length} optional image${r.missing.length > 1 ? "s" : ""} missing; ${game} uses its own.`);
  lines.push(...r.warnings);
  $("notes").replaceChildren(...lines.map((l) => h("li", {}, l)));
}

// Draws the stage to scale as the target game lays it out. Playfield units are
// fractions of the screen height on a 16:9 screen (x runs 0..16/9).
//
// The view always fills the preview box: it's centred on the stage at full
// screen height, and zooms out (keeping the bottom of the screen in view)
// when the stage is too wide to fit, so changing a value never resizes it.
function renderPlayfield(pf) {
  const screen = $("screen");
  const area = $("preview-box").getBoundingClientRect();
  const aspect = area.width / area.height || 0.6;
  const first = pf.columns[0], last = pf.columns.at(-1);
  const x0 = first?.x ?? 0, x1 = (last?.x ?? 0) + (last?.w ?? 0);
  const W = Math.max(aspect, (x1 - x0) * 1.16); // view width, in screen heights
  const H = W / aspect;                          // view height, in screen heights
  const left = (x0 + x1) / 2 - W / 2, top = 1 - H;

  const pct = (v) => `${(v * 100).toFixed(3)}%`;
  const px = (x) => pct((x - left) / W);
  const py = (y) => pct((y - top) / H);
  const ph = (hgt) => pct(hgt / H);
  const out = (path) => `/out/${state.preview.gen}/${path.split("/").map(encodeURIComponent).join("/")}`;
  const img = (src, y, hgt, cls) => {
    const el = h("img", { src: out(src), alt: "", draggable: "false", class: cls });
    el.style.top = py(y);
    el.style.height = ph(hgt);
    return el;
  };
  const box = (cls, left, width, y, hgt) => {
    const el = h("div", { class: cls });
    Object.assign(el.style, { left, width, ...(y != null && { top: py(y) }), ...(hgt != null && { height: ph(hgt) }) });
    return el;
  };

  screen.classList.toggle("over", pf.receptorsOver);
  state.playfield = pf;

  const stage = [px(x0), pct((x1 - x0) / W)];
  const lanes = pf.columns.map((c, i) => {
    const lane = box("lane", px(c.x), pct(c.w / W));
    if (c.receptor) lane.append(img(c.receptor, c.receptorY, c.receptorH, "receptor"));
    if (c.note) {
      // Two notes per lane, offset in time, falling until their bottom edge
      // reaches the hit line.
      for (const k of [0, 1]) {
        const fall = h("div", { class: "fall" }, img(c.note, pf.hitY - c.noteH, c.noteH));
        fall.style.setProperty("--dur", "1.6s");
        fall.style.setProperty("--delay", `${-((i * 0.37 + k * 0.8) % 1.6).toFixed(2)}s`);
        lane.append(fall);
      }
    }
    return lane;
  });

  const sprite = (s, images) => {
    if (!s.image) return null;
    const el = box("sprite", ...stage, s.y - s.h / 2, s.h);
    el.append(...images.map((src) => h("img", { src: out(src), alt: "", draggable: "false" })));
    return el;
  };
  const digits = pf.combo.image ? [..."727"].map((d) => pf.combo.image.replace(/0(\.png)$/i, `${d}$1`)) : [];

  screen.replaceChildren(
    box("stage-bg", ...stage),
    ...lanes,
    box("hitline", ...stage, pf.hitY),
    sprite(pf.judgement, [pf.judgement.image]) ?? "",
    sprite(pf.combo, digits) ?? "",
  );
}

// ---------- saving ----------

async function save() {
  if (state.busy || !state.preview) return;
  const button = $("convert");
  setBusy(true);
  button.classList.add("saving");
  status("Saving…");
  try {
    const saved = await api().Save({
      name: $("in-name").value,
      author: $("in-author").value,
      output: $("in-output").value,
      archive: $("in-archive").checked,
      install: $("in-install").checked,
    });
    const links = [["Show folder", () => api().Reveal(saved.folder)]];
    if (saved.archive) links.push([saved.installed ? `Open in ${saved.game} again` : `Open in ${saved.game}`, () => api().Reveal(saved.archive)]);
    status(saved.installed ? `Saved, and opening in ${saved.game}.` : `Saved to ${saved.folder}`, { links });
    flashDone(saved.installed ? "Installing…" : "Saved");
  } catch (e) {
    status(errText(e), { error: true });
  } finally {
    button.classList.remove("saving");
    setBusy(false);
  }
}

// Briefly swaps the button's label for a confirmation.
let doneTimer;
function flashDone(text) {
  const button = $("convert"), label = $("convert-label");
  clearTimeout(doneTimer);
  button.classList.remove("done");
  void button.offsetWidth; // restart the animation
  button.classList.add("done");
  label.textContent = `${text} ✓`;
  doneTimer = setTimeout(() => {
    button.classList.remove("done");
    label.textContent = `Convert to ${state.skin.to}`;
  }, 1800);
}

// ---------- wiring ----------

function debounce(fn, ms) {
  let t;
  return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); };
}

function showTab(output) {
  $("tab-layout").setAttribute("aria-selected", String(!output));
  $("tab-output").setAttribute("aria-selected", String(output));
  $("pane-layout").hidden = output;
  $("pane-output").hidden = !output;
  syncResetAll();
}

function wire() {
  $("banner").addEventListener("click", openPicker);
  $("picker-search").addEventListener("input", () => { picker.active = 0; renderPicker(); });
  $("picker-search").addEventListener("keydown", pickerKeys);
  $("browse-file").addEventListener("click", () => browse(false));
  $("browse-folder").addEventListener("click", () => browse(true));
  document.addEventListener("pointerdown", (e) => {
    if (picker.open && !e.target.closest(".banner-wrap")) closePicker();
  });

  $("convert").addEventListener("click", save);
  $("reset-all").addEventListener("click", () => resetFields(changedIn(state.mode)));
  $("tab-layout").addEventListener("click", () => showTab(false));
  $("tab-output").addEventListener("click", () => showTab(true));
  $("pick-output").addEventListener("click", async () => {
    try {
      const dir = await api().PickOutput($("in-output").value);
      if (dir) $("in-output").value = dir;
    } catch (e) { status(errText(e), { error: true }); }
  });
  $("in-archive").addEventListener("change", syncInstall);
  for (const id of ["in-grades", "in-hitsounds", "in-healthcw"]) $(id).addEventListener("change", convert);
  $("in-name").addEventListener("input", () => { $("skin-name").textContent = $("in-name").value || state.skin?.name; });
  $("in-author").addEventListener("input", () => { $("skin-author").textContent = $("in-author").value; });

  // External links open in the browser, not in the app window.
  for (const a of document.querySelectorAll('a[href^="https://"]')) {
    a.addEventListener("click", (e) => { e.preventDefault(); window.runtime.BrowserOpenURL(a.href); });
  }

  $("win-min").addEventListener("click", () => window.runtime.WindowMinimise());
  $("win-close").addEventListener("click", () => window.runtime.Quit());

  // Drag & drop: Wails hands over real paths; the highlight is just feedback.
  let depth = 0;
  const isFiles = (e) => e.dataTransfer?.types?.includes("Files");
  const dragging = (on) => document.body.classList.toggle("dragging", on);
  window.addEventListener("dragenter", (e) => { if (isFiles(e)) { depth++; dragging(true); } });
  window.addEventListener("dragleave", (e) => { if (isFiles(e) && --depth <= 0) { depth = 0; dragging(false); } });
  window.addEventListener("drop", () => { depth = 0; dragging(false); });
  window.runtime.OnFileDrop((_x, _y, paths) => {
    depth = 0;
    dragging(false);
    if (paths?.length) load(paths[0]);
  }, false);

  window.addEventListener("keydown", (e) => {
    const mod = e.ctrlKey || e.metaKey;
    if (mod && e.key.toLowerCase() === "o") { e.preventDefault(); openPicker(); }
    if (mod && e.key === "Enter") { e.preventDefault(); save(); }
  });
  new ResizeObserver(() => state.playfield && renderPlayfield(state.playfield)).observe($("preview-box"));
}

async function init() {
  try {
    document.documentElement.dataset.platform = (await window.runtime.Environment()).platform;
  } catch { /* styling falls back to the defaults */ }
  wire();
  renderEmptyRows();
  try {
    const { version, open } = await api().Startup();
    $("title").textContent = `qoconv ${version}`;
    if (open) await load(open); // a skin passed on the command line / dropped on the exe
  } catch (e) {
    status(errText(e), { error: true });
  }
}

// The Wails runtime loads before this module, but bindings attach
// asynchronously; wait for them.
(function ready() {
  if (window.go?.gui?.App && window.runtime) init();
  else setTimeout(ready, 10);
})();
