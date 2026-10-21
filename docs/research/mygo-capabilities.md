# MyGo v0.6.2 — Capability Reference for a Native-UI API Client

Read-only survey of `github.com/egoist/mygo v0.6.2`, the version pinned in this
project's `go.mod` (CLI pinned as `tool github.com/egoist/mygo/cmd/mygo`). Module
root: `/Users/liyunfu/go/pkg/mod/github.com/egoist/mygo@v0.6.2`; every path below
is relative to it unless prefixed `apigo/`. Symbols were verified against source
with `go doc` and `ui/api_gen.go` (generated, 6,116 lines), not only prose docs.
Things that do not exist are marked **[absent]**.

**Verdict in one paragraph.** MyGo supplies the desktop shell (windows, app and
window menus, system dialogs, clipboard, keychain, notifications, file watching,
deep links, single instance, signed updates, packaging), a genuinely broad,
GPU-drawn widget set with virtualization, a Zed-class code editor and diff
viewer, Markdown and rich text, and a headless tester. It supplies **no HTTP
client, WebSocket client, JSON tree widget, scripting runtime, git, or native-UI
printing**. The `fetch`/`websocket` plugins are JS-facing services for web pages,
not Go APIs. So an API client built on `ui` must own the request engine,
`.bru` handling, scripting, JSON inspection and all persistence beyond window
state — which matches the package split already sketched in `apigo/AGENTS.md`
(`internal/httpx`, `internal/collection`, `internal/app`, `internal/ui`).

---

## 1. App, windows, menus, documents

### 1.1 Application lifecycle and identity

| Need                           | API (exact symbol)                                                                                 | Source                  | Notes                                                          |
| ------------------------------ | -------------------------------------------------------------------------------------------------- | ----------------------- | -------------------------------------------------------------- |
| Run loop                       | `mygo.App.Run() error`                                                                             | `app.go`                | Main goroutine only; returns when the app quits                |
| Create first window            | `mygo.App.WhenReady(func())`                                                                       | `app.go`                | Also runs immediately if already ready                         |
| Quit hooks                     | `OnBeforeQuit(func(*mygo.QuitEvent))`, `OnWillQuit`, `OnQuit`                                      | `app.go`                | `e.PreventDefault()` cancels; useful to flush tabs/collections |
| Keep app alive with no windows | `App.OnWindowAllClosed(func(){})`                                                                  | `docs/app.md`           | Otherwise last window closes → app quits                       |
| Re-activate (macOS Dock)       | `App.OnActivate(func(hasVisibleWindows bool))`                                                     | `app.go`                | Reopen main window                                             |
| App identity                   | `App.SetName`, `App.SetVersion`, `App.Name()`, `App.Version()`, `App.IsPackaged()`, `mygo.IsDev()` | `app.go`                | `mygo dev` app is `<Name> Dev`, identifier `<identifier>.dev`  |
| Dock / taskbar                 | `App.SetBadgeCount`, `App.Dock.SetBadge/Bounce/SetIcon/SetMenu/Hide/Show`, `App.JumpList.SetTasks` | `app.go`, `jumplist.go` | Badge = running requests; jump-list tasks = "New Request"      |

Windows exist only after ready; all APIs are goroutine-safe, while listeners
and view builds run on the main thread.

### 1.2 Windows

`mygo.NewWindow(mygo.WindowOptions{...}) *mygo.Window` (`window.go`).

| Option / method                                           | Use for an API client                                                                      |
| --------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `Content: ui.View(app.view)`                              | Native UI; mutually exclusive with `URL`                                                   |
| `Title`, `Width`, `Height`, `MinWidth/MinHeight`          | Main window                                                                                |
| `StateKey: "main"`                                        | Remembers position/size/maximized in `window-state.json`                                   |
| `Parent`, `Modal`                                         | Preferences sheet                                                                          |
| `Panel: true`, `Frameless`, `Hidden`                      | Command palette (see `docs/windows.md#panels`)                                             |
| `TitleBarStyle: mygo.TitleBarHidden`, `HideWindowButtons` | Custom title bar drawn with `c.TitleBar()` + `Element.DragWindow()` (`docs/ui/windows.md`) |
| `Vibrancy` + `c.Vibrancy()`                               | Sidebar material on macOS                                                                  |

Methods: `SetTitle`, `Close`/`Destroy`, `Show/Hide/Focus`, `SetMenu`, `OnClose`
(veto with `e.PreventDefault()`), `OnClosed`, `OnResize/OnMove`, `OnFileDrop`,
`Update(func())`, `Invalidate()`, `SetRepresentedFile`, `SetDocumentEdited`,
`SetProgressBar`, `FlashFrame`, `CapturePage() ([]byte, error)` (PNG, native UI
too), `NativeHandle()` + `mygo.RunOnMain`; find with `mygo.Windows()`,
`mygo.FocusedWindow()`, `mygo.WindowByID(id)`. `Window.Update` is the only
sanctioned way for a request goroutine to publish results.

### 1.3 Menus and shortcuts

`mygo.NewMenu([]*mygo.MenuItem{...})` (`menu.go`). `MenuItem` fields: `ID`,
`Label`, `Type` (`MenuItemNormal/Checkbox/Radio/Separator/Submenu`), `Role`,
`Accelerator`, `Disabled`, `Hidden`, `Checked`, `ToolTip`, `Submenu`, `Click`.
Live edits: `SetLabel/SetEnabled/SetVisible/SetChecked/SetAccelerator`,
`Menu.Append/Insert/ItemByID`, `mygo.Separator()`.

| Need                | API                                                                                     |
| ------------------- | --------------------------------------------------------------------------------------- |
| App menu bar        | `mygo.App.SetMenu(menu)`; macOS app-wide, Linux/Windows per window via `Window.SetMenu` |
| Default menus       | `RoleAppMenu`, `RoleFileMenu`, `RoleEditMenu`, `RoleViewMenu`, `RoleWindowMenu`         |
| Document items      | `RoleNew/Open/OpenRecent/Save/SaveAs/Duplicate/Rename/MoveTo/Revert`                    |
| Editing             | `RoleUndo/Redo/Cut/Copy/Paste/Delete/SelectAll/PasteAndMatchStyle`                      |
| Window              | `RoleClose`, `RoleMinimize`, `RoleZoom`, `RoleToggleFullScreen`                         |
| Context menu        | `Menu.Popup(win)` (blocks until closed) / `PopupAt(win,x,y)`                            |
| Tray                | `mygo.NewTray(mygo.TrayOptions{...})` (Linux needs libayatana-appindicator3)            |
| Accelerators        | `"CmdOrCtrl+Shift+P"`, `"CmdOrCtrl+Enter"`; same syntax as `GlobalShortcut`             |
| In-window shortcuts | `c.Shortcut(mods,key)`, `Element.Shortcut`, `Handle.OnShortcut(c,...)`, `c.OnShortcut`  |

Caveat: menu accelerators are app/window scoped. Per-tab commands like
"Send active request" must be routed in the view (`c.Shortcut`) to the active
tab, or declared on a `ui.Handle` bound to the request pane.

### 1.4 Document model (window-per-file, autosave, recovery, watchers)

`mygo.NewDocuments(mygo.DocumentsOptions[T]{...}) *mygo.Documents[T]`, called
before `Run` (`document.go`, `docs/documents.md`). Generic over content type.

| Field / method                                                                                                                                                                | Behaviour                                                                                                |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `Types []DocumentType`, `Read`, `Write`, `Window func(*Document[T]) *Window`, `New`, `Equal`, `Snapshot`                                                                      | One window per document; app builds content; dirty comparison; snapshot before async write               |
| `Autosave` (`AutosaveInPlace` default, `AutosaveOff`), `AutosaveDelay` (5s)                                                                                                   | Writes ~5s after last edit, on blur, on close, ≥ every 30s while typing                                  |
| `Document[T]`: `Content`, `Set`, `Changed`, `SetEdited`, `State`, `Save`, `SaveAs`, `Duplicate`, `Rename`, `MoveTo`, `Revert`, `Close`, `Window`, `OnReload`, `OnStateChange` | Full NSDocument-like surface                                                                             |
| `DocumentState{Name, Path, Edited, ReadOnly, Conflict, Deleted}`                                                                                                              | Drive tab title, dirty dot, conflict banner                                                              |
| Recovery copies                                                                                                                                                               | `Recovery/<kind>` in `PathUserData`; restored after crash/forced quit, per-session lock                  |
| External changes                                                                                                                                                              | Documents watch their files and re-check on focus; `OnReload`, `Conflict`, `Deleted`; rename is followed |

**Fit for a tabbed client.** Documents is _window-per-file_, not tab-per-file.
Shape 1: one window per collection file (`Documents[Collection]`) with request
tabs _inside_ it — autosave, recovery, external changes, recent docs, file
associations and dirty chrome come free; multiple collections are multiple
windows. Shape 2: one window with a custom tab model across collections — you
re-implement all of the above, but may still use `SetRepresentedFile` /
`SetDocumentEdited`. §9 assumes Shape 1.

### 1.5 Single instance, deep links, file associations, recent docs

| Need                           | API / config                                                                                                                                                                         | Notes                                                                                                                                                |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| One instance                   | `App.RequestSingleInstanceLock() bool` + `App.OnSecondInstance(func(args []string, workingDir string))`                                                                              | Second instance passes argv and exits; required on Windows/Linux so file opens reach the first instance                                              |
| Deep links / file associations | `urlSchemes` + `fileAssociations` in `mygo.json`; `App.OnOpenURL(func(string))`, `App.OnOpenFile(func(path))`, `App.RegisterURLScheme`/`UnregisterURLScheme`/`IsURLSchemeRegistered` | Register before `Run` to get the launch URL/file. Good for OAuth callbacks (`apigo://oauth/callback`) and claiming `.bru`; pair with single-instance |
| Recent docs                    | `App.AddRecentDocument(path)`, `App.RecentDocuments() []string` (newest first, max 10, existing only), `App.ClearRecentDocuments()`; `RoleOpenRecent`                                | Linux writes `recently-used.xbel`; Windows uses jump list; macOS Dock                                                                                |
| Window state / login item      | `WindowOptions.StateKey` → `window-state.json`, or `App.SetWindowStateStore`; `App.SetOpenAtLogin`/`OpenAtLogin`/`WasOpenedAtLogin`                                                  | Loaded at first keyed window, written on close/quit                                                                                                  |

---

## 2. Native UI toolkit inventory

Package `ui` is immediate-mode-ish: `ui.View(func(*ui.Context))` is a window's
`Content`; the view rebuilds every frame from app state
(`ui/doc.go`, `docs/ui/README.md`, `docs/ui/views.md`). Constructors return
`ui.Element`; those values expire before the next build pass. Keep data,
`ui.ListState`/`editor.State`, and `ui.Handle` in app state — never elements.

### 2.1 Layout and containers

| Need                        | API                                                                                                                                                 | Source                               |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------ |
| Flex rows/columns           | `ui.Row`, `ui.Column`, `ui.Box`, `.Gap/.Padding/.Margin`, `.Grow(1)`, `.Fill/.FillWidth/.FillHeight`, `ui.Spacer`, `ui.Divider`, `ui.Dividers`      | `ui/api_gen.go`, `docs/ui/layout.md` |
| CSS-grid layout             | `ui.Grid`, `.Columns/.Rows/.ColumnSpan/.RowSpan`                                                                                                    | `ui/grid.go`, `docs/ui/grid.md`      |
| Scrolling                   | `ui.Scroll`, `ui.ScrollHorizontal`, `ui.ScrollBoth`, `Element.ScrollIntoView`, `ScrollWith`                                                         | `ui/scroll.go`                       |
| **Split panes**             | `ui.Split(c,&w,first,second)`, `ui.SplitVertical`, `ui.SplitPercent`, `ui.SplitVerticalPercent`; divider drag + arrow keys, 40 DIP min, `Changed()` | `docs/ui/split.md`                   |
| Absolute/attached placement | `.Absolute().Top/Right/Bottom/Left`, `.Attach(at,self)`, `.AttachTo(anchor,...)`                                                                    | `docs/ui/layout.md`                  |
| Clip / hide                 | `.Clip()`, `.ClipX()`, `.ClipY()`, `.Invisible()`, `.PassThrough()`                                                                                 | `ui/api_gen.go`                      |
| Custom title bar            | `c.TitleBar()` + `.DragWindow()`, `.MinimizeButton()`, `.MaximizeButton()`                                                                          | `docs/ui/windows.md`                 |

### 2.2 Navigation, collections, virtualization

| Need                            | API                                                                                                                                       | Verified limits                                                                                                  |
| ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Source list / sidebar           | `ui.Sidebar(c,&selected,fn)` + `ui.SidebarItem(c,id,icon,label)` + `ui.SidebarSection(c,title,&open,fn)`                                  | String IDs; icon `*ui.SVG`; rows are sections/items, not arbitrary rows                                          |
| Tabs (tab bar)                  | `ui.Tabs(c,&index,labels...)`; custom look: `ui.TabsBase(c,&index,n)` → `TabsParts.List`, `TabsParts.Tab(i)`                              | Labels only; **no built-in close button, dirty dot, drag-reorder or overflow menu** — build in `Tab(i).Children` |
| Tree (small)                    | `ui.Tree(c,fn)` + `ui.TreeItem(c,label,&open,children)`                                                                                   | Builds **every** item; use only for small trees                                                                  |
| Virtualized tree / table-tree   | `ui.Outline[K](c,&state,roots,children,row)`, `ui.OutlineTable[K]`; `OutlineState[K]{List ListState, Open Selection[K], Indent}`          | Builds only visible rows; `Rows()`, `Item(row)`, `Depth(row)`                                                    |
| Virtualized list                | `ui.List(c,&state,n,row)`, `ListState.Key`, `Element.ItemKey(func(int) any)`, `Rows(func(ui.ListRow))`                                    | Builds visible rows + margin; variable row heights; millions of rows                                             |
| Virtualized table               | `ui.Table(c,&state,cols,n,cell)`; `TableColumn{Title,ID,Width,MinWidth,MaxWidth,Align,Sortable,Fixed,RowHeader}`                          | Visible rows only; sticky section headers; row height = tallest cell                                             |
| Column resize/move              | built into `ui.Table`; persist `ListState.Columns TableLayout{Order,Widths}`                                                              | Restore from disk, save on change                                                                                |
| Sorting                         | `ListState.Sort *SortOrder`, `SortBy *[]SortOrder`, `SortOrder{Column,Descending}`                                                        | **App sorts the slice**; table only reports and draws arrows                                                     |
| Multi-select                    | `ListState.Selected *int`, `Selection Selector`, `ui.Selection[K]` (`Add/Remove/Has/Len/All/Clear`), `DragSelect`                         | Cmd/Shift semantics built in                                                                                     |
| Cell block select (spreadsheet) | `ListState.CellSelect` + `ListState.Cells ColumnRange`; `CellSelected(row,col)`, `SelectedColumns`, `SelectColumns`                       | Not for `OutlineTable` columns (arrows keep opening rows)                                                        |
| Reorder by drag                 | `ListState.Reorder func(rows []int, to int)`                                                                                              | Mutually exclusive with `DragSelect`                                                                             |
| Context menus in collections    | `ListState.HeaderMenu`, `CellMenu`; `Element.ContextMenu(func(*ui.Menu))`; `m.Item/Checked/Disabled/Shortcut/Separator/Submenu/EditItems` | System-drawn menus                                                                                               |
| Filter box                      | `ui.SearchField(c,&query)`, `ui.FindBar(c,&open,&query,matches,&current)`                                                                 | Filter on `Changed`                                                                                              |
| Rename in place                 | `ui.EditableText(c,&name)` inside a table cell                                                                                            | Return / F2                                                                                                      |

### 2.3 Code, text and document views

| Need                 | API                                                                                                                                                                                                       | Notes                                                                                                                                                                                                                  |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Code editor          | `editor.View(c,*editor.State)`; `editor.State{Buffer *editor.Buffer, ReadOnly, SoftWrap, LineNumbers, TabSize, Theme, Bindings, ...}`                                                                     | `ui/editor/build.go`, `docs/ui/code-editor.md`                                                                                                                                                                         |
| Buffer               | `editor.NewBuffer(text)`, `Buffer.Set/Edit/Transact/SetLanguage/Modified/MarkSaved/WriteTo/Version/Snapshot`, `Buffer.Undo/Redo`                                                                          | Rope-like persistent tree; 1M-line files; snapshots safe from goroutines                                                                                                                                               |
| Highlighting         | `editor.LanguageFor("x.json")`, `LanguageNamed`, `Languages()`; `editor.Language{Name,Extensions,LineComment,Tokenizer,Highlighter}`; `editor.TokenizerFunc`; `NewTokenHighlighter`                       | Languages: Go, JS, TS, **JSON**, Python, Rust, C, C++, Java, Kotlin, Swift, C#, Shell, YAML, TOML, SQL, Ruby, Lua, PHP, CSS, HTML, XML, Markdown, Diff, Dockerfile, Makefile, Protobuf, Zig (`ui/editor/languages.go`) |
| LSP-ish hooks        | `State.OnCompletion/ShowCompletions`, `OnHover/ShowHover`, `SetDiagnostics(buf.Version(),...)`, `SetInlays`, `SetSuggestion`, `OnDefinition`, `OnInput`                                                   | App answers; can be async via `Window.Update`                                                                                                                                                                          |
| Editor commands      | `State.Run(editor.Command)`, `editor.CommandNamed("selectNext")`, `editor.DefaultBindings`, `editor.NoCommand`, `State.Find`, `FindMatches`, `Fold`, `Goto`                                               | Zed keymap                                                                                                                                                                                                             |
| Diff viewer          | `editor.DiffView(c,*editor.DiffState)`; `DiffState{Old,New *editor.Buffer, Layout: DiffSplit/DiffUnified, Inline, Indicators, ContextLines, Editable, HunkActions}`; `Accept/Revert/Hunks/Stats/GoToHunk` | Useful for "changes since last run", collection diffs, response diff                                                                                                                                                   |
| Single-line input    | `ui.TextInput(c,&s)` + `.Placeholder/.Password/.ReadOnly/.Disabled/.Label/.AutoFocus/.OnPaste/.OnTextKey/.TextRanges/.TextSelection`                                                                      | IME, undo, clipboard, context menu                                                                                                                                                                                     |
| Multi-line input     | `ui.TextArea(c,&s)` + `.Lines(min,max)` + `.TrackLines(&ui.TextLines)` + `.TrackScroll`                                                                                                                   | Layouts only visible paragraphs; hundreds of thousands of lines                                                                                                                                                        |
| Large read-only body | `ui.TextArea(c,&raw).ReadOnly(true)` or `editor.View` with `ReadOnly: true`                                                                                                                               | Prefer editor for JSON/XML; TextArea for plain text                                                                                                                                                                    |
| Markdown             | `markdown.View(c,src,&markdown.Options{})`, `markdown.Stream`; `Options.LoadImage`, `Options.Code`, `Options.Theme`, `Options.OnLink`                                                                     | CommonMark + GFM; fenced code highlighted with editor tokenizers; good for docs/README and API descriptions                                                                                                            |
| Markdown parser      | `ui/markdown/parser.Parse/Append/Edit`, `Document.HTML()`                                                                                                                                                 | No `ui` dependency; safe in engine packages                                                                                                                                                                            |
| Rich text display    | `ui.RichText(c, spans...)`                                                                                                                                                                                | Styled runs, links                                                                                                                                                                                                     |
| Rich text editing    | `richtext.Editor(c,*richtext.Document)`; `NewDocument`, `NewText`, `TextOf`, `ParseMarkdown/HTML/RTF`, `Markdown()/HTML()`                                                                                | Overkill for JSON, useful for notes/docs                                                                                                                                                                               |

### 2.4 Choices, dialogs, overlays, feedback

| Need                 | API                                                                                                                          |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Dropdown             | `ui.Select(c,&s,options)`; typed `ui.SelectBase[T](c,&v)` with `Trigger/Popup/Item/Highlight`                                |
| Filtering combobox   | `ui.Combobox(c,&s,options)`; `ui.ComboboxBase`                                                                               |
| Autocomplete         | `ui.Autocomplete(c,&s,suggestions)`                                                                                          |
| Chips / token field  | `ui.TokenField(c,&tokens,suggestions)` — env vars, tags, header names                                                        |
| Checkbox / partial   | `ui.Checkbox(c,&b,label)`, `ui.CheckboxGroup`                                                                                |
| Toggle / switch      | `ui.Toggle(c,&b,label)`, `ui.ToggleGroup`, `ui.Switch(c,&b)`                                                                 |
| Radio group          | `ui.Radio[T](c,&v,value,label)`, `ui.RadioGroup`                                                                             |
| Number / date / time | `ui.NumberInput`, `ui.DateInput`, `ui.TimeInput`, `ui.Calendar`, `ui.ColorPicker`                                            |
| Modal / alert        | `ui.Modal(c,&open,fn)`, `ui.DialogBase`, `Dismissed()` to veto close; `ui.AlertDialog(c,&open,title,message,buttons...) int` |
| Popover              | `ui.Popover(c,anchor,&open,fn)`; `ui.PopoverBase`; `AttachTo`                                                                |
| Free overlay         | `ui.Overlay(c,fn)`, `Element.AttachTo`, `PressedOutside`                                                                     |
| Context menu         | `Element.ContextMenu(fn)`, `Element.Menu(fn)`, `ui.MenuButton(c,label,fn)`                                                   |
| Tooltip              | `Element.Tooltip(text)`, `Element.Description(text)`, `ui.TooltipBase(c,anchor,fn)`                                          |
| Toast                | `c.Toast(msg)`, `c.ToastAction(msg,action,fn)`, `c.AddToast(ui.Toast{...})`, `c.CloseToast(id)`, `ui.ToastViewportBase`      |
| Progress             | `ui.Progress(c,0..1)`; negative = indeterminate                                                                              |
| Spinner              | `ui.Spinner(c)`                                                                                                              |
| Meter / badge        | `ui.Meter(c,v,lo,hi,levels)`, `ui.Badge(c,text)`                                                                             |

### 2.5 Theming, icons, drag/drop, focus, a11y, i18n

| Area                    | API                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Theme                   | `c.Theme() *ui.Theme`; fields `Background`, `Surface`, `SurfaceHover/Pressed`, `Border`, `Text`, `TextMuted`, `Accent`, `AccentText`, `Danger`, `Warning`, `Success`, `Selection`, `Focus`, `Inverse/InverseText`, `Radius`, `Spacing`, `FontSize`, `Scrollbar`, `ScrollbarWidth`; helpers `t.Space(n)`, `t.Rem(n)`; `c.SetTheme(&t)`; `ui.LightTheme`, `ui.DarkTheme`; colors `ui.RGB`, `ui.RGBA`, `ui.Hex`, `ui.Oklch`, `Mix`, `Alpha` |
| Desktop-following       | `c.Preferences()` (`Accent`, `HighContrast`, `TextScale`, `ReduceMotion`); `mygo.Theme.SetSource(mygo.ThemeDark/Light/System)`; theme follows automatically                                                                                                                                                                                                                                                                              |
| Icons / images          | `ui.Icon(c,*ui.SVG)`, `ui.ParseSVG`, `ui.MustParseSVG` (color = text color; Lucide/Heroicons/Tabler work); `ui.Image(c, ui.ImageSource)` with `*ui.Bitmap` (`NewBitmap`, `DecodeBitmap`) or SVG                                                                                                                                                                                                                                          |
| Drag within window      | `Element.Drag(value)`, `ui.Drop[T](e)`, `ui.DragOver[T](e)`, `Element.Dragging()`                                                                                                                                                                                                                                                                                                                                                        |
| Drag to/from other apps | `Element.DragData(transfer.Data, transfer.DragOptions{...})`, `ui.DropData`, `ui.DataDragOver`, `transfer.TextData/URLData/FileData/Lazy/Bytes`                                                                                                                                                                                                                                                                                          |
| Files dropped           | `Element.DroppedFiles()`, `Element.FileDragOver()`, `Window.OnFileDrop`                                                                                                                                                                                                                                                                                                                                                                  |
| Focus                   | `Element.Focusable/AutoFocus/Focus/Focused/FocusVisible/FocusWithin`, `Element.Bind(&h)`, `Handle.Focus/CancelFocus/Focused/Bounds/OnShortcut`, `c.Key`, `Element.Key`, `FocusBind(&field,value)`, `FocusedValue(c,&field)`, `FocusGroup(ui.Horizontal)`                                                                                                                                                                                 |
| Shortcuts               | `c.Shortcut(mods,key)`, `Element.Shortcut`, `Handle.OnShortcut`, `c.OnShortcut`, `ui.Cmd` (⌘/Ctrl), physical-key mapping for non-US layouts                                                                                                                                                                                                                                                                                              |
| Accessibility           | Widgets expose roles/names/states automatically; `Label`, `Description`, `Role(ui.RoleSwitch)`, `Checked/Expanded/Value/Range/Step/Level/ActiveDescendant`, `c.Announce(text)`; drag-and-drop and list/table row counts are exposed                                                                                                                                                                                                      |
| i18n                    | Catalogs (`i18n.dir`, `source`) → generated `locales.X(args)`; `locales.In("de").X()`; `ui.Context.Locale()/Region()`; `Element.Lang("ja")`; RTL layout follows locale; MyGo's own strings/menus are translated; `c.Announce`, documents' prompts localized                                                                                                                                                                              |

**Styling is code, not CSS** (`docs/ui/styling.md`): `.Background`, `.Border`,
`.Radius`, `.Shadow`, `.InsetShadow`, `.Gradient`, `.Blur`, `.Opacity`,
`.TextColor`, `.FontSize`, `.Bold`, `.Cursor`; no selectors, style follows state
(`if row.Hovered() { ... }`).

---

## 3. Desktop APIs

All in package `mygo` (`docs/native.md`), safe from any goroutine; calls that
need a running app panic if made in `main` before `Run`.

| Capability               | API                                                                                                                                                                                                                                                                                                                 | Notes for an API client                                                                                 |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| Open/save file dialogs   | `mygo.Dialog.Open(mygo.OpenDialogOptions{Parent,Title,Filters,Directory,Multiple,ShowHiddenFiles,...}) ([]string,error)`; `Dialog.Save(SaveDialogOptions) (string,error)`                                                                                                                                           | Blocking: call from goroutine, then `win.Update`                                                        |
| Message dialogs          | `mygo.Dialog.Message(MessageOptions{Type,Message,Detail,Buttons,DefaultButton,CancelButton,CheckboxLabel}) (MessageResult,error)`; `Dialog.Error(title,msg)`                                                                                                                                                        | `MessageWarning/Question/...`                                                                           |
| Clipboard                | `ReadText/WriteText`, `ReadImage/WriteImage(png)`, `WriteHTML`, `ReadFiles/WriteFiles`, `AvailableFormats`, `Clear`; rich: `Write(transfer.Data)`, `Read(formats...)`, `Formats/ReadFormat/Flush`                                                                                                                   | "Copy as cURL", "Copy body", "Paste headers"                                                            |
| Notifications            | `mygo.NewNotification(NotificationOptions{ID,Title,Subtitle,Body,Silent,Group,Sender}).Show()`, `.Close()`, `.OnClick(fn)`; `App.OnNotificationClick(id)`, `mygo.NotificationsSupported()`, `ClearNotifications()`                                                                                                  | Request finished/failed while app is in background                                                      |
| Secrets / keychain       | `mygo.Secrets.Set/Get/Delete(key, value []byte)`, `Secrets.Service("name")`, `ErrSecretNotFound`, `MaxSecretSize = 2560` bytes, key ≤ 256 bytes                                                                                                                                                                     | Store API tokens/refresh tokens; **2560-byte cap** — encrypt larger blobs yourself                      |
| Shell                    | `Shell.OpenExternal(url)`, `OpenPath(path)`, `ShowItemInFolder`, `TrashItem`, `Beep`, `FileIcon(path,size)`, `InstalledApps()`, `LaunchApp(app)`                                                                                                                                                                    | OAuth in browser, reveal collection file                                                                |
| Network status           | `mygo.Network.Status() mygo.NetworkStatus{Online,Metered}`, `Network.OnChanged(fn)`                                                                                                                                                                                                                                 | Disable Send offline; show "metered" warning                                                            |
| Watch files              | `mygo.WatchFiles(ctx, path, mygo.FileWatchOptions{Recursive,Ignore,Debounce,MaxDelay,MaxEntries,HintsOnly}) (*mygo.FileWatcher,error)`; `w.Next(ctx) ([]mygo.FileEvent,error)`, `w.Close()`, `w.Root()`; `FileEvent{Op,Path,OldPath,IsDir}`, `FileCreate/FileWrite/FileRemove/FileRename/FileHint`, `ErrWatchLimit` | Collection-level watchers (Documents already watch their own file)                                      |
| Global shortcuts         | `mygo.GlobalShortcut.Register(accel, fn)`, `Unregister`, `UnregisterAll`, `IsRegistered`                                                                                                                                                                                                                            | Quick-open request palette; Wayland needs installed app + portal                                        |
| Updater                  | `mygo.Updater.Enabled()`, `Check(ctx) (*Update,error)`, `Update.Install(ctx, progress func(done,total int64))`, `Version/Notes/Date`, `ErrUpdatesDisabled`                                                                                                                                                          | Signed, delta-capable                                                                                   |
| Cookies / browsing data  | `mygo.Cookies.List(CookieFilter)/Set/Delete/Clear`, `App.ClearBrowsingData()`                                                                                                                                                                                                                                       | Only the **webview** cookie store (OAuth logins in a `ui.WebView`); not `net/http` jar                  |
| Displays / power / audio | `mygo.Screen` (displays, cursor, `OnDisplaysChanged`); `mygo.Power.KeepAwake/OnSuspend/IsOnBattery/IdleTime`; package `audio` (`DecodeSound`, `OpenStream`, players)                                                                                                                                                | Palette placement, long runs, optional completion sound                                                 |
| Theme / appearance       | `mygo.Theme.IsDark/OnUpdated/SetSource/Source/Preferences`                                                                                                                                                                                                                                                          | Persist appearance choice; apply before `Run`                                                           |
| Printing / PDF           | `Page.Print()`, `Page.PrintToPDF(PDFOptions)`, **web pages only**; `Window.CapturePage()` PNG for native UI                                                                                                                                                                                                         | **[absent]** native-UI PDF/print. Export via HTML in a hidden WebView, or render JSON to HTML and print |
| Native escape hatch      | `win.NativeHandle()` (`NSWindow*`/`GtkWindow*`/`HWND`), `mygo.RunOnMain(fn)`                                                                                                                                                                                                                                        | Only for what MyGo doesn't wrap                                                                         |

---

## 4. HTTP and networking story

**MyGo provides no public HTTP client.** The module imports `net/http`
internally (protocol handlers, updater, downloads, CLI publishing) but exports
only:

| Export                                                                                             | What it is                                                                                                                                                 | Where                             |
| -------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------- |
| `mygo.FileServer(fsys fs.FS) http.Handler`                                                         | Serves an `fs.FS` to webview pages                                                                                                                         | `protocol.go`                     |
| `mygo.Protocol.Handle(scheme string, h http.Handler) error`, `HandleFunc`, `Unhandle`, `IsHandled` | Registers custom URL schemes (`app://`) served by Go to **webview pages**; handlers run on their own goroutines; schemes must be registered before windows | `protocol.go`, `docs/frontend.md` |
| `mygo.SetFrontend(fs.FS)`                                                                          | Embeds a frontend for `mygo://` pages                                                                                                                      | `frontend.go`                     |
| `mygo.Bind/BindAs/NewEvent`                                                                        | Typed Go↔TS IPC for pages (not native UI)                                                                                                                  | `ipc.go`, `docs/bindings.md`      |
| `Page.Eval`, `mygo.EvalAs[T]`                                                                      | Run JS in a webview page                                                                                                                                   | `window.go`                       |

There is no `mygo.HTTP`, request builder, Go cookie jar, WebSocket dialer, SSE
helper, proxy API, or local HTTP server API.

### 4.1 What the plugins actually are

`docs/plugins.md`: a plugin = Go service + npm JS package, called from **pages**.

| Plugin                                         | JS surface                                                                 | Go-side                                                                                         | Native-UI usable?                                                    |
| ---------------------------------------------- | -------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| `plugins/fetch`                                | `fetch()` with CORS-free requests, any header, streamed body, cancellation | `fetch.New(fetch.Options{Client *http.Client, Allow func(*http.Request) bool})`, `fetch.Plugin` | **No** — service is registered for pages; no exported Go request API |
| `plugins/websocket`                            | browser-like `WebSocket` + handshake headers                               | `websocket.New(Options{Client,Allow,MaxMessageSize,CloseTimeout})`                              | **No** — same                                                        |
| `plugins/sqlite`                               | `open/execute/query/transaction`                                           | **Yes**: `sqlite.Open(ctx, name, sqlite.OpenOptions{})`, `db.Execute/Query/Transaction/Close`   | Yes, and no `mygo.Use` needed                                        |
| `plugins/watch`                                | watch named roots                                                          | Yes, but `mygo.WatchFiles` is simpler                                                           | Yes                                                                  |
| `plugins/updater`                              | Sparkle-like update window                                                 | `updater.Plugin`                                                                                | `native.Plugin` variant exists                                       |
| `plugins/terminal`, `glass`, `game`, `loading` | —                                                                          | native UI (terminal uses Ghostty VT + `mygo-plugin.json` native lib)                            | Yes                                                                  |

Constraints (even if unused): trusted pages only; `mode/credentials/cache/
referrerPolicy/integrity/keepalive/priority` ignored; `Set-Cookie` unreadable;
cookies from the Go jar, not the webview; non-http(s) falls back to webview
fetch; Go waits while >1 MiB is unread; `Allow` sees only the first request; WS
has no permessage-deflate, 64 MiB messages, 5 s close handshake
(`docs/plugins/{fetch,websocket}.md`).

### 4.2 What the app must build (native UI)

- **HTTP engine** on `net/http`: `http.Client` with `Timeout`, custom
  `Transport` (`Proxy: http.ProxyFromEnvironment`, TLS config, client certs,
  HTTP/2), `cookiejar.Jar`, `CheckRedirect`, `httptrace` for timings,
  `mime/multipart` for uploads, `context.WithCancel` for Cancel, `bufio` for
  SSE/NDJSON. Publish via `win.Update`; never block the view.
- **Streaming** read `resp.Body` in chunks, throttle updates (16–33 ms), cap
  retained bytes, offer "save to file" for huge bodies. **WebSocket** needs a Go
  library (`coder/websocket`, `gorilla/websocket`); MyGo wraps none for Go.
- **Proxy** only via `http.Transport` (`HTTPS_PROXY`, `NO_PROXY`); no system
  proxy API. `http.DefaultClient` reads environment variables only.
- **Scripting**: **[absent]**. No JS runtime exists for native UI; the only JS
  engine is the system webview (`ui.WebView` / `win.NewWebView`). Pre-request
  and test scripts must be implemented in Go (own interpreter, `goja`, etc.) or
  sandboxed inside a hidden WebView page — with the caveat that page JS cannot
  reach Go except through `mygo.Bind` methods you expose.
- **CGO**: MyGo is pure Go (`purego`/`syscall`); `mygo dev/build/generate`
  default to `CGO_ENABLED=0` (set `1` only for app deps that need it). SQLite is
  prebuilt and loaded via purego, so metadata storage stays CGO-free.
- **Browser (`GOOS=js GOARCH=wasm`)**: same UI via WebGL 2, but menus, trays,
  notifications, global shortcuts, file dialogs, non-file drags,
  `Protocol.Handle/FileServer`, bindings and the updater do not exist
  (`docs/ui/browsers.md`). Treat desktop as the target.
- **Local HTTP server** (OAuth loopback callback): build it with `net/http`
  yourself; `Protocol` is a webview scheme registry, not a socket listener.

---

## 5. Data persistence and configuration

| Need                        | API / location                                                                                                                                                              | Notes                                                                                                                                                        |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| User data root              | `App.Path(mygo.PathUserData)`                                                                                                                                               | macOS `~/Library/Application Support/<Name>`, Linux `~/.config/<Name>`, Windows `%AppData%\<Name>`; created on use                                           |
| Cache / logs / resources    | `PathCache`, `PathLogs`, `PathResources`                                                                                                                                    | `PathResources` = bundled `resources/` (or `Contents/Resources`)                                                                                             |
| Other dirs                  | `PathAppData`, `PathHome`, `PathTemp`, `PathExe`, `PathDesktop/Documents/Downloads/Pictures/Music/Videos`                                                                   | `App.SetPath` overrides (tests)                                                                                                                              |
| Window geometry             | `WindowOptions.StateKey`; `window-state.json`; `App.SetWindowStateStore`                                                                                                    | Store interface: `LoadWindowStates`/`SaveWindowStates`                                                                                                       |
| App preferences             | **[absent]** generic store                                                                                                                                                  | Write JSON with atomic rename into `PathUserData`; or SQLite                                                                                                 |
| Collections / history       | SQLite plugin: `sqlite.Open(ctx, "apigo.db", sqlite.OpenOptions{})`; `sqlite.Options{Directory, BusyTimeout}`; `db.Execute/Query/Transaction`; `sqlite.Statement{SQL,Args}` | No cgo; results buffered; `ATTACH`, extensions, multiple statements rejected; files confined to `Directory`; `:memory:` supported (`docs/plugins/sqlite.md`) |
| Documents                   | `Recovery/<kind>`, `recent-documents.json`, `window-state.json` under `PathUserData`                                                                                        | Written by MyGo                                                                                                                                              |
| Locale / dev data / updater | `locale` file (Linux/Windows), macOS defaults; `.mygo/dev` + `<name> Dev` identity; `updates` in `mygo.json` + `mygo/update-keys`                                           | See §6                                                                                                                                                       |

Secrets must go to `mygo.Secrets` (OS keychain), not files — matching
`apigo/AGENTS.md` rule 5.

---

## 6. Packaging, release, CLI

`mygo.json` (native-UI projects; `mygo.config.ts` has the same fields plus
frontend fields) — verified against `cmd/mygo/config.go`:

| Group       | Fields                                                                                                                                                                                     |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| App         | `name`, `identifier`, `version`, `copyright`, `icon`, `main`, `out`                                                                                                                        |
| Integration | `resources`, `urlSchemes`, `fileAssociations[{ext,name,mimeType,role}]`                                                                                                                    |
| i18n        | `i18n.dir`, `i18n.source`                                                                                                                                                                  |
| Updates     | `updates.publicKey`, `github` or `url`, `tagPrefix` (`v`), `s3{bucket,prefix,region,endpoint,pathStyle}`, `privateKey`, `changelog`, `deltas` (3)                                          |
| macOS       | `macos.icon`, `minimumSystemVersion` (12.0), `signingIdentity`, `entitlements`, `provisioningProfile`, `helperEntitlements`, `infoPlist`, `dmgTitle`, `notarize{keychainProfile,keychain}` |
| Windows     | `windows.certificate`, `signCommand`, `timestampUrl`                                                                                                                                       |
| Linux       | `linux.maintainer`, `comment`, `categories`, `depends`, `webview` (set `false` for pure native UI → GTK only, no WebKitGTK), `command`                                                     |

Build outputs (`go tool mygo build`, `docs/distribution.md`):

| Platform | `build/<os>-<arch>/` (or `out`)                                                                        |
| -------- | ------------------------------------------------------------------------------------------------------ |
| macOS    | `<Name>.app` signed; `<Name> <version> <arch>.dmg` (or `.dmg` for `darwin/universal`)                  |
| Windows  | `<Name>.exe`, app files, `<Name> Setup <version> <arch>.exe` (NSIS; per-user install)                  |
| Linux    | binary, `.desktop`, `.png`, MIME `.xml`, `<name>-<version>-linux-<arch>.tar.gz` + `install.sh`, `.deb` |
| Web      | `<out>/web`: `index.html`, `wasm_exec.js`, hashed `.wasm`                                              |

Flags: `-platform darwin/universal,windows/amd64,linux/arm64,web`, `-debug`,
`-o`, `-sign`, `-skip-dmg`, `-skip-notarize`, `-skip-build-command`,
`-upload`.

Signing: ad-hoc by default (runs only on the build Mac); set
`macos.signingIdentity` + `notarize.keychainProfile` (from `xcrun notarytool
store-credentials`). Windows: `certificate` (.pfx, password in
`MYGO_WINDOWS_CERTIFICATE_PASSWORD`) or `signCommand` (Azure Trusted Signing).
Helpers/resources are signed too; Linux builds are unsigned.

Updates: `mygo keygen` → `mygo-update.key` (never commit) + `.pub` in
`updates.publicKey`; `MYGO_UPDATER_PRIVATE_KEY` at build; build writes
`my-app-<v>-<platform>.tar.gz`, `update-<platform>.json`, and deltas from up to
`updates.deltas` prior versions; `-upload` to a GitHub draft or S3; apps verify
signatures; release notes from `CHANGELOG.md` `## <version>`.

GitHub Actions: `setup-go` with `go-version-file: go.mod`; no Bun for native UI
— `go tool mygo build -platform "${{ matrix.platform }}" -upload`; macOS runner
for sign/notarize/DMG, Ubuntu for Windows+Linux; secrets
`MYGO_UPDATER_PRIVATE_KEY`, `MYGO_WINDOWS_CERTIFICATE_PASSWORD`, notary creds;
`contents: write` for drafts; check `jq -r .version mygo.json`.

CLI pinning: `go.mod` has `tool github.com/egoist/mygo/cmd/mygo` (Go 1.24+
tool directive), so `go tool mygo dev|build|vet|generate|i18n|keygen|doctor`
runs exactly v0.6.2; `mygo init -template native` creates this layout;
`mygo install-skills` refreshes `.agents/skills/`.

---

## 7. Testing

| Concern            | Answer                                                                                                                                                                                                                       |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Headless UI tests  | `ui.NewTester(view func(*ui.Context), width, height int) *ui.Tester` (`ui/api_gen.go`, `ui/headless.go`) renders frames in memory, no window, no GPU                                                                         |
| Find/click/type    | `tt.Click("Send")`, `tt.ClickAt`, `tt.ClickWith(ui.Cmd,...)`, `tt.Type`, `tt.Key(ui.Cmd,ui.KeyS)`, `tt.TypeKey`, `tt.Scroll/Wheel/Pinch`, `tt.RightClick`, `tt.Menu`, `tt.ChooseMenuItem`                                    |
| Assertions         | `tt.Texts()`, `tt.HasText`, `tt.Find(s) (Rect,bool)`, `tt.Focused`, `tt.Cursor`, `tt.Clipboard`, `tt.OpenedURLs`, `tt.Announcements`, `tt.Image() *image.RGBA`                                                               |
| Deterministic time | `tt.Advance(d)` drives animations, `c.After`, tooltips, toasts; `tt.Frame()`, `tt.FramePending()`; never `time.Sleep`                                                                                                        |
| Desktop variations | `tt.SetSize`, `tt.SetScale`, `tt.SetDark`, `tt.SetPreferences`, `tt.SetDirection`, `tt.SetLocale`, `tt.SetRegion`, `tt.SetVibrancy`, `tt.SetTitleBar`                                                                        |
| WebView in tests   | `tt.WebViewAt(x,y)`, `tt.ClickAt` over it (`docs/ui/web-view.md`)                                                                                                                                                            |
| One-shot render    | `ui.Render(view, w, h, scale) *image.RGBA` for golden snapshots                                                                                                                                                              |
| Real-GUI e2e       | **[absent for apps]**. `MYGO_E2E=1 go test ./internal/e2e` is inside the framework repo (`internal/`), needs a macOS desktop session, and is not importable by apps. App-level e2e means driving the built binary externally |
| Vet                | `go tool mygo vet .` runs `go vet ./...` plus `internal/uivet` lifetime analysis: flags `ui.Element`/parts stored in structs, package vars, or captured by goroutines (`cmd/mygo/vet_ui.go`, `internal/uivet/analyze.go`)    |
| CGO                | `CGO_ENABLED=0 go test ./...` is the required gate (no cgo anywhere in MyGo)                                                                                                                                                 |

Verified in this workspace during the survey (module cache offline):

```text
GOPROXY=off go test ./...                 → ok apigo 0.316s
CGO_ENABLED=0 GOPROXY=off go test ./...   → ok apigo
GOPROXY=off go tool mygo vet .            → exit 0, no diagnostics
```

Caveat: `go doc`/`go test` for the `ui` package wants test-only modules
(`go-text/typesetting-utils`, `x/tools`, …); with no proxy they may time out.
Vendoring or a warm module cache is needed for offline CI.

---

## 8. Gaps and risks for an API client

Legend: **G** = MyGo gives it, **B** = app must build it, **R** = risk.

| Area                                                            | Status  | Detail / mitigation                                                                                                                                                                                                                                                          |
| --------------------------------------------------------------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| HTTP engine, auth, redirects, cookies, proxy, TLS, client certs | **B**   | `net/http` + `cookiejar` + `Transport`; MyGo has no engine. Keep it in `internal/httpx`, test with `httptest`                                                                                                                                                                |
| WebSocket / SSE / gRPC                                          | **B**   | SSE via `bufio`; WS needs a Go dependency; gRPC needs `google.golang.org/grpc` (pure Go, CGO off)                                                                                                                                                                            |
| Scripting (pre-request, tests, `bru` API, `req`/`res`)          | **B**   | No JS runtime outside a webview. Options: Go expression engine, `goja`, or hidden WebView + `mygo.Bind`. Must enforce timeouts/no fs/no net per project security rules                                                                                                       |
| `.bru` parse/print, variable interpolation, assertions, runner  | **B**   | Pure Go packages; MyGo only supplies files, dialogs, watchers, UI                                                                                                                                                                                                            |
| JSON tree viewer                                                | **B**   | No JSON widget. Use `ui.Outline[NodeID]` (virtualized) with `OutlineState.Open`, or `ui.Tree` for small payloads. `encoding/json` into `any` only at the view boundary                                                                                                       |
| Very large responses                                            | **R/G** | G: `ui.List`/`ui.Table`/`ui.Outline` virtualize rows; `editor` lays out only visible lines; `TextArea` handles 100k+ lines. R: building a full JSON tree or pretty string in memory. Cap preview (e.g. 5–10 MB), stream to a temp file, offer "Open in editor" / "Save body" |
| Syntax highlighting for HTTP/GraphQL/.env                       | **R**   | `editor` ships 28 languages, **no GraphQL, HTTP, .env, HCL, protobuf-text**. Add `editor.Language{Tokenizer: editor.TokenizerFunc}` (cheap line tokenizer) or accept JSON/plain                                                                                              |
| Editor and diff                                                 | **G**   | Multi-caret, folding, find/replace, completions, diagnostics, hovers, million-line buffers, `ReadOnly`, `DiffBase`, custom bindings; `editor.DiffView` adds split/unified, hunk actions, annotations                                                                         |
| Tabs UX                                                         | **R/B** | `ui.Tabs` is a label strip. Close buttons, dirty dots, middle-click, drag-reorder, overflow and per-tab context menus need `ui.TabsBase` + `Tab(i).Children` + `ui.ContextMenu`/`Drag`. No native tab bar                                                                    |
| Multi-window vs tabs                                            | **R**   | `Documents` is window-per-file. If the product wants one window with N collection tabs, build the tab/file model and lose Documents' autosave/recovery. Recommended: collection = document window, requests = tabs (§9)                                                      |
| Table sorting/filtering                                         | **B**   | `ui.Table` reports sort state; the app sorts. Filtering/pagination also app-side                                                                                                                                                                                             |
| Tree for large collections                                      | **G**   | `ui.Outline`/`OutlineTable`; `ui.Tree` builds everything                                                                                                                                                                                                                     |
| Native-UI printing/PDF                                          | **B**   | Only web pages print (`Page.PrintToPDF`); native UI yields PNG via `CapturePage`. Export by rendering an HTML report in a hidden WebView, or generate PDF in Go                                                                                                              |
| Git integration                                                 | **B**   | No git API/plugin. `os/exec` `git` or a Go git library; respect "no UI imports outside internal/ui"                                                                                                                                                                          |
| Secrets size                                                    | **R**   | `MaxSecretSize` 2560 bytes, key 256; larger blobs need app-side encryption with a key in `Secrets`                                                                                                                                                                           |
| Preferences / proxy                                             | **B/R** | No config store (JSON/SQLite it); no system-proxy API, `http.Transport` only, no PAC or cert UI                                                                                                                                                                              |
| Cookies / OAuth loopback                                        | **R/B** | `mygo.Cookies` is the webview store only (engine uses `cookiejar`); no local server API — bind `net/http` on `127.0.0.1`, or use `OnOpenURL` deep links                                                                                                                      |
| Accessibility of custom widgets                                 | **R**   | Built-ins are well exposed; hand-drawn rows need `Label`/`Role`/`Value`/`Level` or screen readers see nothing                                                                                                                                                                |
| Keyboard model                                                  | **R**   | Menu accelerators are window/app-wide. Route per-tab shortcuts through `c.Shortcut`/`Handle.OnShortcut`; test both macOS ⌘ and Ctrl                                                                                                                                          |
| i18n                                                            | **G**   | Catalogs + typed functions + RTL + localized framework strings; don't hard-code English                                                                                                                                                                                      |
| Browser build                                                   | **R**   | wasm build loses file dialogs, menus, notifications, global shortcuts, secrets (unsupported), protocol handlers. Treat desktop as the target                                                                                                                                 |
| Framework churn                                                 | **R**   | 0.x: read `docs/ui/migration.md` on every minor bump and run `go tool mygo migrate-ui .`; pin the CLI via `go.mod` tool                                                                                                                                                      |
| Startup / size                                                  | **G/R** | No webview: low memory, immediate first frame; `mygo build` strips the inspector, but SQLite/terminal/glass add native libs and size                                                                                                                                         |

**Bottom line:** MyGo is strong exactly where a Bruno-like native client is weak
today (desktop integration, virtualized views, editor, theming, a11y, packaging,
updates) and silent exactly where the engine lives (HTTP, scripting, `.bru`,
git). Plan for `internal/httpx`, `internal/script`, `internal/collection`,
`internal/bru` to be ordinary Go packages with no `ui` import, tested with
`httptest` and golden files.

---

## 9. Recommended composition for the main window

State that outlives frames (never keep `ui.Element`, parts, or `*ui.Context`):

```go
type app struct {
    coll     ui.OutlineState[*Node]  // virtualized sidebar
    filter   string
    tabs     []*RequestTab
    active   int
    reqTab   int                     // Params | Headers | Body | Auth | Scripts
    respTab  int                     // Body | Headers | Cookies | Tests
    sidebarW, responseH, bodySplit float32
    respBody ui.OutlineState[JSONNode]
    respHead ui.ListState
    run      *RunState               // Cancel, StartedAt, generation
    win      *mygo.Window
}
type RequestTab struct {
    ID, Title  string
    Req        *Request
    Params     ui.ListState          // table columns/sort
    ParamRows  []Param
    Body       *editor.State         // one buffer per tab = undo history
    Resp       *Response
    RespEditor *editor.State
    Cancel     context.CancelFunc
    Gen        uint64
}
```

```go
func (a *app) view(c *ui.Context) {
    ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
        a.toolbar(c)                                 // environment picker + Send
        ui.Split(c, &a.sidebarW,
            func() { a.sidebar(c) },
            func() {
                ui.SplitVerticalPercent(c, &a.responseH,
                    func() { a.requestPane(c) },
                    func() { a.responsePane(c) }).Grow(1)
            }).Grow(1)
        a.statusBar(c)                               // timing, size, progress
    })
}

func (a *app) sidebar(c *ui.Context) {
    ui.Column(c).Width(240).FillHeight().Children(func() {
        ui.SearchField(c, &a.filter).Label("Filter")
        ui.Outline(c, &a.coll, a.roots(),
            func(n *Node) []*Node { return n.Children },
            func(n *Node) {
                row := ui.Row(c).Gap(6).Padding(2, 4)
                ui.Icon(c, n.Icon)
                ui.Text(c, n.Name).SingleLine().Grow(1)
                row.ContextMenu(func(m *ui.Menu) { /* New / Rename / Delete */ })
            }).Grow(1)
    })
}
```

Request pane — `TabsBase` because `Tabs` has no close button; a table of params;
one JSON editor per tab:

```go
func (a *app) requestPane(c *ui.Context) {
    tab := a.tabs[a.active]
    tabs := ui.TabsBase(c.Key("req-tabs"), &a.active, len(a.tabs))
    tabs.List.Gap(2).Children(func() {
        for i := range a.tabs {
            item := tabs.Tab(i).Padding(6, 10).Radius(6)
            if i == a.active { item.Background(c.Theme().Surface) }
            item.Children(func() {
                ui.Text(c, a.tabs[i].Title).SingleLine()
                if ui.Button(c, "").Label("Close").Width(18).Clicked() { a.closeTab(i) }
            })
        }
    })
    ui.Row(c).Gap(8).Padding(8).Children(func() {
        ui.Select(c, &tab.Req.Method, methods).Width(96).Label("Method")
        ui.TextInput(c, &tab.Req.URL).Label("URL").Grow(1).
            OnSubmit(func() { a.send(tab) })
        if ui.PrimaryButton(c, "Send").Clicked() { a.send(tab) }
    })
    ui.Tabs(c, &a.reqTab, "Params", "Headers", "Body", "Auth", "Scripts")
    if a.reqTab == 2 {                               // Body
        editor.View(c, tab.Body).Grow(1).FillWidth()
        return
    }
    ui.Table(c, &tab.Params, []ui.TableColumn{
        {Title: "", ID: "on", Width: 36, Fixed: true},
        {Title: "Key", ID: "key"}, {Title: "Value", ID: "value"},
    }, len(tab.ParamRows), func(row, col int) {
        p := &tab.ParamRows[row]
        switch col {
        case 0: ui.Checkbox(c, &p.Enabled, "")
        case 1: ui.TextInput(c, &p.Key).Placeholder("key")
        case 2: ui.TextInput(c, &p.Value).Placeholder("value")
        }
    }).Grow(1)
}
```

Response pane — JSON tree beside the read-only pretty body:

```go
func (a *app) responsePane(c *ui.Context) {
    tab := a.tabs[a.active]
    ui.Row(c).Gap(8).Padding(6, 10).Children(func() {
        if tab.Resp == nil { ui.Text(c, "No response").TextColor(c.Theme().TextMuted); return }
        ui.Badge(c, tab.Resp.Status)
        ui.Textf(c, "%d ms · %s", tab.Resp.Milliseconds, humanBytes(tab.Resp.Size))
        ui.Spacer(c)
        if ui.Button(c, "Copy as cURL").Clicked() { a.copyCurl(tab) }
    })
    ui.Tabs(c, &a.respTab, "Body", "Headers", "Cookies", "Tests")
    switch a.respTab {
    case 0:
        ui.Split(c, &a.bodySplit,
            func() { ui.Outline(c, &a.respBody, tab.JSONRoot(), jsonChildren, jsonRow).Grow(1) },
            func() { editor.View(c, tab.RespEditor).ReadOnly(true).Grow(1) }).Grow(1)
    case 1:
        ui.Table(c, &a.respHead, []ui.TableColumn{
            {Title: "Header", Sortable: true}, {Title: "Value", Sortable: true},
        }, len(tab.Resp.Headers), func(row, col int) { /* ui.Text */ }).Grow(1)
    }
}
```

Rules: **documents** — one `mygo.NewDocuments[Collection]` window per
collection file, request tabs inside it, so autosave/recovery/recent/file
associations come free. **Persistence** — save `ui.Split` sizes and
`ListState.Columns` with the collection. **Threading** — `send` uses
`context.WithCancel`, increments `tab.Gen`, publishes through
`win.Update(func(){...})`, drops stale generations, cancels on tab close.
**Keys** — `c.Key(tab.ID+":url")` for stateful controls; `*editor.State` and
`ui.ListState` live in `RequestTab`. **Palette** — `Panel:true` window +
`GlobalShortcut.Register("CmdOrCtrl+Shift+P", ...)` + `SearchField` + `List`.
**Settings** — `Parent`/`Modal` window with `StateKey:"settings"`, `ui.Tabs`,
`ui.Switch`, `mygo.Secrets`.

### Source index

Root: `app.go`, `window.go`, `menu.go`, `document*.go`, `single_instance.go`,
`deeplink.go`, `recent_docs.go`, `window_state.go`, `dialog.go`, `clipboard.go`,
`secrets.go`, `network.go`, `watch.go`, `updater.go`, `cookies.go`, `power.go`,
`pdf.go`, `protocol.go`, `paths.go`, `modules.go`, `page.go`, `webview.go`.
UI: `ui/api_gen.go`, `ui/doc.go`, `ui/{editor,markdown,richtext}/*`,
`ui/{split,table,list,outline,sidebar,theme,headless,preferences,ref}.go`.
CLI: `cmd/mygo/{config.go,vet_ui.go}`, `internal/uivet/analyze.go`.
