package cli

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"opc-xml-da-cli/service"
)

const maxTUILogLines = 200

type tuiNode struct {
	ID          string
	Label       string
	ItemPath    string
	ItemName    string
	IsItem      bool
	HasChildren bool
}

type tuiAttribute struct {
	Name  string
	Value string
}

type tuiValue struct {
	ID    string
	Label string
	Value string
}

type tuiBackend interface {
	Children(ctx context.Context, node tuiNode) ([]tuiNode, error)
	Details(ctx context.Context, node tuiNode) ([]tuiAttribute, error)
	Read(ctx context.Context, node tuiNode) (tuiValue, error)
	Watch(ctx context.Context, nodes []tuiNode, interval time.Duration) (<-chan tuiValue, <-chan error, func(), error)
}

type xmlDATUIBackend struct {
	svc          service.OpcXmlDASoap
	locale       string
	clientHandle string
}

func (b *xmlDATUIBackend) Children(ctx context.Context, node tuiNode) ([]tuiNode, error) {
	elements, err := fetchBrowseElements(ctx, b.svc, b.locale, b.clientHandle, node.ItemPath, node.ItemName)
	if err != nil {
		return nil, err
	}
	nodes := make([]tuiNode, 0, len(elements))
	for _, el := range elements {
		if el == nil {
			continue
		}
		nodes = append(nodes, xmlBrowseElementToTUI(el))
	}
	sort.Slice(nodes, func(i, j int) bool {
		return strings.ToLower(nodes[i].Label) < strings.ToLower(nodes[j].Label)
	})
	return nodes, nil
}

func (b *xmlDATUIBackend) Details(_ context.Context, node tuiNode) ([]tuiAttribute, error) {
	return []tuiAttribute{
		{Name: "Name", Value: node.Label},
		{Name: "ItemPath", Value: node.ItemPath},
		{Name: "ItemName", Value: node.ItemName},
		{Name: "IsItem", Value: fmt.Sprint(node.IsItem)},
		{Name: "HasChildren", Value: fmt.Sprint(node.HasChildren)},
	}, nil
}

func (b *xmlDATUIBackend) Read(ctx context.Context, node tuiNode) (tuiValue, error) {
	if node.ItemPath == "" && node.ItemName == "" {
		return tuiValue{}, errors.New("read requires an item path or item name")
	}
	resp, err := FetchNodeValue(ctx, b.svc, b.locale, b.clientHandle, node.ItemPath, node.ItemName)
	if err != nil {
		return tuiValue{}, err
	}
	value := tuiValue{ID: node.ID, Label: node.Label, Value: "<no value>"}
	if resp != nil && resp.RItemList != nil && len(resp.RItemList.Items) > 0 && resp.RItemList.Items[0] != nil {
		item := resp.RItemList.Items[0]
		value.ID = xmlItemKey(item.ItemPath, item.ItemName)
		value.Label = firstNonEmpty(item.ItemName, item.ItemPath, node.Label)
		value.Value = formatXMLDAValue(item.Value)
		if quality := formatOPCQuality(item.Quality); quality != "" {
			value.Value += " [" + quality + "]"
		}
	}
	return value, nil
}

func (b *xmlDATUIBackend) Watch(ctx context.Context, nodes []tuiNode, interval time.Duration) (<-chan tuiValue, <-chan error, func(), error) {
	values := make(chan tuiValue, 32)
	errs := make(chan error, 8)
	watchCtx, cancel := context.WithCancel(ctx)
	go func() {
		defer close(values)
		defer close(errs)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			for _, node := range nodes {
				value, err := b.Read(watchCtx, node)
				if err != nil {
					select {
					case errs <- err:
					case <-watchCtx.Done():
						return
					}
					continue
				}
				select {
				case values <- value:
				case <-watchCtx.Done():
					return
				}
			}
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return values, errs, cancel, nil
}

func xmlBrowseElementToTUI(el *service.BrowseElement) tuiNode {
	label := browseElementName(el)
	return tuiNode{
		ID:          xmlItemKey(el.ItemPath, el.ItemName),
		Label:       label,
		ItemPath:    el.ItemPath,
		ItemName:    el.ItemName,
		IsItem:      el.IsItem,
		HasChildren: el.HasChildren,
	}
}

type tuiController struct {
	backend   tuiBackend
	interval  time.Duration
	monitored map[string]tuiValue
	nodes     map[string]tuiNode
	logs      []string

	watchCancel context.CancelFunc
	watchStop   func()
	mu          sync.Mutex
}

func newTUIController(backend tuiBackend, interval time.Duration) *tuiController {
	return &tuiController{
		backend:   backend,
		interval:  interval,
		monitored: map[string]tuiValue{},
		nodes:     map[string]tuiNode{},
	}
}

func (c *tuiController) addLog(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = append(c.logs, line)
	if len(c.logs) > maxTUILogLines {
		c.logs = append([]string(nil), c.logs[len(c.logs)-maxTUILogLines:]...)
	}
}

func (c *tuiController) logText() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.logs, "\n")
}

func (c *tuiController) setMonitored(node tuiNode, value tuiValue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes[node.ID] = node
	c.monitored[node.ID] = value
}

func (c *tuiController) setValue(value tuiValue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.monitored[value.ID] = value
}

func (c *tuiController) removeMonitored(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.nodes, id)
	delete(c.monitored, id)
}

func (c *tuiController) monitoredNodes() []tuiNode {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]string, 0, len(c.nodes))
	for id := range c.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	nodes := make([]tuiNode, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, c.nodes[id])
	}
	return nodes
}

func (c *tuiController) monitoredValues() []tuiValue {
	c.mu.Lock()
	defer c.mu.Unlock()
	values := make([]tuiValue, 0, len(c.monitored))
	for _, value := range c.monitored {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].Label < values[j].Label
	})
	return values
}

func (c *tuiController) stopWatch() {
	if c.watchCancel != nil {
		c.watchCancel()
		c.watchCancel = nil
	}
	if c.watchStop != nil {
		c.watchStop()
		c.watchStop = nil
	}
}

func (c *tuiController) restartWatch(ctx context.Context, onValue func(tuiValue), onError func(error)) error {
	c.stopWatch()
	nodes := c.monitoredNodes()
	if len(nodes) == 0 {
		return nil
	}
	watchCtx, cancel := context.WithCancel(ctx)
	values, errs, stop, err := c.backend.Watch(watchCtx, nodes, c.interval)
	if err != nil {
		cancel()
		return err
	}
	c.watchCancel = cancel
	c.watchStop = stop
	go func() {
		for {
			select {
			case <-watchCtx.Done():
				return
			case value, ok := <-values:
				if !ok {
					return
				}
				c.setValue(value)
				onValue(value)
			case err, ok := <-errs:
				if !ok {
					return
				}
				onError(err)
			}
		}
	}()
	return nil
}

func (a *App) tui(args []string) error {
	opts := defaultCommandOptions()
	interval := time.Second
	fs := a.newFlagSet("tui")
	addCommonFlagsWithoutFormat(fs, &opts)
	fs.StringVar(&opts.BrowsePath, "item-name", "", "OPC browse item name")
	fs.StringVar(&opts.BrowseItemPath, "item-path", "", "OPC browse item path")
	fs.DurationVar(&interval, "interval", interval, "poll interval for monitored values")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if interval <= 0 {
		return fmt.Errorf("--interval must be greater than zero")
	}
	if err := opts.applyConfig(fs); err != nil {
		return err
	}
	if err := configureLogging(opts); err != nil {
		return err
	}
	_, opcService, err := a.newService(opts)
	if err != nil {
		return err
	}
	root := tuiNode{
		ID:          xmlItemKey(opts.BrowseItemPath, opts.BrowsePath),
		Label:       firstNonEmpty(opts.BrowsePath, opts.BrowseItemPath, "<root>"),
		ItemPath:    opts.BrowseItemPath,
		ItemName:    opts.BrowsePath,
		HasChildren: true,
	}
	backend := &xmlDATUIBackend{svc: opcService, locale: opts.Locale, clientHandle: opts.ClientHandle}
	return runTUI(context.Background(), "OPC XML-DA Browser (polling)", root, backend, interval)
}

func runTUI(ctx context.Context, title string, root tuiNode, backend tuiBackend, interval time.Duration) error {
	app := tview.NewApplication()
	controller := newTUIController(backend, interval)
	tree := tview.NewTreeView()
	details := tview.NewTable().SetBorders(false)
	monitored := tview.NewTable().SetBorders(false)
	logView := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	footer := tview.NewTextView().SetDynamicColors(true).SetText("Enter:Expand  tab:Next  a:Attributes  r:Read  m:Monitor  u:Unmonitor  R:Reload  c:Clear  ?:Help  q:Exit")

	rootNode := tview.NewTreeNode(root.Label).SetReference(root).SetColor(tcell.ColorGreen)
	tree.SetRoot(rootNode).SetCurrentNode(rootNode)
	styleBox(tree.Box, "Address Space")
	styleBox(details.Box, "Attribute List")
	styleBox(monitored.Box, "Monitored Items")
	styleBox(logView.Box, "Info")

	controller.addLog(title + " ready")
	refreshLog := func() {
		logView.SetText(controller.logText())
		logView.ScrollToEnd()
	}
	refreshLog()

	refreshMonitored := func() {
		monitored.Clear()
		monitored.SetCell(0, 0, tview.NewTableCell("Item").SetTextColor(tcell.ColorAqua).SetSelectable(false))
		monitored.SetCell(0, 1, tview.NewTableCell("Value").SetTextColor(tcell.ColorAqua).SetSelectable(false))
		for i, value := range controller.monitoredValues() {
			monitored.SetCell(i+1, 0, tview.NewTableCell(value.Label))
			monitored.SetCell(i+1, 1, tview.NewTableCell(value.Value))
		}
	}
	refreshMonitored()

	showDetails := func(node *tview.TreeNode) {
		ref, ok := currentTUINode(node)
		if !ok {
			return
		}
		attrs, err := backend.Details(ctx, ref)
		if err != nil {
			controller.addLog("attributes " + ref.ID + ": " + err.Error())
			refreshLog()
			return
		}
		details.Clear()
		for i, attr := range attrs {
			details.SetCell(i, 0, tview.NewTableCell(attr.Name).SetTextColor(tcell.ColorAqua))
			details.SetCell(i, 1, tview.NewTableCell(attr.Value))
		}
		controller.addLog("attributes refreshed for " + ref.ID)
		refreshLog()
	}

	loadChildren := func(node *tview.TreeNode, force bool) {
		ref, ok := currentTUINode(node)
		if !ok {
			return
		}
		if !force && len(node.GetChildren()) > 0 {
			node.SetExpanded(!node.IsExpanded())
			return
		}
		children, err := backend.Children(ctx, ref)
		if err != nil {
			controller.addLog("browse " + ref.ID + ": " + err.Error())
			refreshLog()
			return
		}
		node.ClearChildren()
		for _, child := range children {
			label := child.Label
			if child.IsItem {
				label += "  item"
			}
			if child.HasChildren {
				label += "  /"
			}
			childNode := tview.NewTreeNode(label).SetReference(child)
			if child.HasChildren {
				childNode.SetColor(tcell.ColorWhite)
			}
			node.AddChild(childNode)
		}
		node.SetExpanded(true)
		controller.addLog(fmt.Sprintf("loaded %d children for %s", len(children), ref.ID))
		refreshLog()
	}

	readSelected := func(node *tview.TreeNode) {
		ref, ok := currentTUINode(node)
		if !ok {
			return
		}
		value, err := backend.Read(ctx, ref)
		if err != nil {
			controller.addLog("read " + ref.ID + ": " + err.Error())
			refreshLog()
			return
		}
		controller.addLog(fmt.Sprintf("value %s read as %s", value.Label, value.Value))
		refreshLog()
	}

	monitorSelected := func(node *tview.TreeNode) {
		ref, ok := currentTUINode(node)
		if !ok {
			return
		}
		controller.setMonitored(ref, tuiValue{ID: ref.ID, Label: firstNonEmpty(ref.Label, ref.ID), Value: "<waiting>"})
		refreshMonitored()
		if err := controller.restartWatch(ctx, func(value tuiValue) {
			app.QueueUpdateDraw(func() {
				refreshMonitored()
				controller.addLog("poll " + value.Label + " changed/read as " + value.Value)
				refreshLog()
			})
		}, func(err error) {
			app.QueueUpdateDraw(func() {
				controller.addLog("poll: " + err.Error())
				refreshLog()
			})
		}); err != nil {
			controller.addLog("monitor " + ref.ID + ": " + err.Error())
			refreshLog()
			return
		}
		controller.addLog("polling " + ref.ID)
		refreshLog()
	}

	unmonitorSelected := func(node *tview.TreeNode) {
		ref, ok := currentTUINode(node)
		if !ok {
			return
		}
		controller.removeMonitored(ref.ID)
		refreshMonitored()
		if err := controller.restartWatch(ctx, func(value tuiValue) {
			app.QueueUpdateDraw(func() {
				refreshMonitored()
				controller.addLog("poll " + value.Label + " changed/read as " + value.Value)
				refreshLog()
			})
		}, func(err error) {
			app.QueueUpdateDraw(func() {
				controller.addLog("poll: " + err.Error())
				refreshLog()
			})
		}); err != nil {
			controller.addLog("poll restart: " + err.Error())
		}
		controller.addLog("unmonitored " + ref.ID)
		refreshLog()
	}

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		loadChildren(node, false)
		showDetails(node)
	})
	tree.SetChangedFunc(showDetails)

	right := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(details, 0, 1, false).
		AddItem(monitored, 0, 1, false)
	top := tview.NewFlex().
		AddItem(tree, 0, 2, true).
		AddItem(right, 0, 3, false)
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(top, 0, 4, true).
		AddItem(logView, 0, 2, false).
		AddItem(footer, 1, 0, false)

	focusables := []tview.Primitive{tree, details, monitored, logView}
	focusIndex := 0
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		node := tree.GetCurrentNode()
		switch event.Key() {
		case tcell.KeyCtrlC:
			controller.stopWatch()
			app.Stop()
			return nil
		case tcell.KeyTab:
			focusIndex = (focusIndex + 1) % len(focusables)
			app.SetFocus(focusables[focusIndex])
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				controller.stopWatch()
				app.Stop()
				return nil
			case 'a':
				showDetails(node)
				return nil
			case 'r':
				readSelected(node)
				return nil
			case 'm':
				monitorSelected(node)
				return nil
			case 'u':
				unmonitorSelected(node)
				return nil
			case 'R':
				loadChildren(node, true)
				return nil
			case 'c':
				controller.logs = nil
				refreshLog()
				return nil
			case '?':
				controller.addLog("keys: arrows/Enter expand, tab focus, a attributes, r read, m monitor, u unmonitor, R reload, c clear, q exit")
				refreshLog()
				return nil
			}
		}
		return event
	})

	loadChildren(rootNode, true)
	showDetails(rootNode)
	if err := app.SetRoot(layout, true).SetFocus(tree).Run(); err != nil {
		controller.stopWatch()
		return err
	}
	controller.stopWatch()
	return nil
}

func currentTUINode(node *tview.TreeNode) (tuiNode, bool) {
	if node == nil {
		return tuiNode{}, false
	}
	ref, ok := node.GetReference().(tuiNode)
	return ref, ok
}

func styleBox(box *tview.Box, title string) {
	box.SetBorder(true).SetTitle(" " + title + " ").SetTitleColor(tcell.ColorAqua)
}

func xmlItemKey(itemPath, itemName string) string {
	if itemPath == "" && itemName == "" {
		return "<root>"
	}
	return itemPath + "\x00" + itemName
}

func formatXMLDAValue(value service.AnyType) string {
	raw := strings.TrimSpace(value.InnerXML)
	if raw == "" {
		return "<empty>"
	}
	if !strings.Contains(raw, "<") {
		return html.UnescapeString(raw)
	}
	return compactXML(raw)
}

func compactXML(raw string) string {
	decoder := xml.NewDecoder(strings.NewReader(raw))
	var out strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			out.WriteByte('<')
			out.WriteString(t.Name.Local)
			out.WriteByte('>')
		case xml.EndElement:
			out.WriteString("</")
			out.WriteString(t.Name.Local)
			out.WriteByte('>')
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				out.WriteString(text)
			}
		}
	}
	if out.Len() == 0 {
		return strings.Join(strings.Fields(raw), " ")
	}
	return out.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
