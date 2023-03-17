package send

import (
	"context"
	"fmt"
	"image/color"
	"sort"

	"code.cryptopower.dev/group/cryptopower/app"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	libutils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
)

const (
	ManualCoinSelectionPageID = "backup_success"

	// MaxAddressLen defines the maximum length of address characters displayed
	// on the UI.
	MaxAddressLen = 16
)

// UTXOInfo defines a utxo record associated with a specific checkbox.
// The record should always retain that checkbox including when rows re-ording
// happens.
type UTXOInfo struct {
	*sharedW.UnspentOutput
	checkbox    cryptomaterial.CheckBoxStyle
	addressCopy *cryptomaterial.Clickable
}

type AccountsUTXOInfo struct {
	Account string
	Details []*UTXOInfo
}

type ManualCoinSelectionPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	actionButton cryptomaterial.Button
	clearButton  cryptomaterial.Button

	selectedUTXOs cryptomaterial.Label
	txSize        cryptomaterial.Label
	totalAmount   cryptomaterial.Label

	selectedUTXOrows []*sharedW.UnspentOutput
	selectedAmount   float64

	amountLabel        labelCell
	addressLabel       labelCell
	confirmationsLabel labelCell
	dateLabel          labelCell

	// UTXO table sorting buttons. Clickable used because button doesn't support
	// cryptomaterial.Image Icons.
	amountClickable        *cryptomaterial.Clickable
	addressClickable       *cryptomaterial.Clickable
	confirmationsClickable *cryptomaterial.Clickable
	dateClickable          *cryptomaterial.Clickable

	accountsUTXOs     []*AccountsUTXOInfo
	UTXOList          *cryptomaterial.ClickableList
	fromCoinSelection *cryptomaterial.Clickable
	collapsibleList   []*cryptomaterial.Collapsible

	listContainer *widget.List
	utxosRow      *widget.List

	lastSortEvent Lastclicked
	clickables    []*cryptomaterial.Clickable
	addressCopy   []*cryptomaterial.Clickable
	properties    []componentProperties

	sortingInProgress bool
	strAssetType      string
}

type componentProperties struct {
	direction layout.Direction
	spacing   layout.Spacing
	weight    float32
}

type Lastclicked struct {
	clicked int
	count   int
	current int
}

type labelCell struct {
	clickable *cryptomaterial.Clickable
	label     cryptomaterial.Label
}

func NewManualCoinSelectionPage(l *load.Load) *ManualCoinSelectionPage {
	pg := &ManualCoinSelectionPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ManualCoinSelectionPageID),

		actionButton: l.Theme.Button(values.String(values.StrDone)),
		clearButton:  l.Theme.OutlineButton("— " + values.String(values.StrClearSelection)),

		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		utxosRow: &widget.List{
			List: layout.List{Axis: layout.Horizontal},
		},
		UTXOList:    l.Theme.NewClickableList(layout.Vertical),
		addressCopy: make([]*cryptomaterial.Clickable, 0),
	}

	pg.actionButton.Font.Weight = text.SemiBold
	pg.clearButton.Font.Weight = text.SemiBold
	pg.clearButton.Color = l.Theme.Color.Danger
	pg.clearButton.Inset = layout.UniformInset(values.MarginPadding3)

	c := l.Theme.Color.Danger
	// Background is 8% of the Danger color.
	alphaChan := (127 * 0.8)
	pg.clearButton.Background = color.NRGBA{c.R, c.G, c.B, uint8(alphaChan)}

	pg.txSize = pg.Theme.Label(values.TextSize14, "--")
	pg.totalAmount = pg.Theme.Label(values.TextSize14, "--")
	pg.selectedUTXOs = pg.Theme.Label(values.TextSize14, "--")

	pg.txSize.Font.Weight = text.SemiBold
	pg.totalAmount.Font.Weight = text.SemiBold
	pg.selectedUTXOs.Font.Weight = text.SemiBold

	pg.fromCoinSelection = pg.Theme.NewClickable(false)

	pg.amountClickable = pg.Theme.NewClickable(true)
	pg.addressClickable = pg.Theme.NewClickable(true)
	pg.confirmationsClickable = pg.Theme.NewClickable(true)
	pg.dateClickable = pg.Theme.NewClickable(false)

	pg.strAssetType = pg.WL.SelectedWallet.Wallet.GetAssetType().String()
	name := fmt.Sprintf("%v(%v)", values.String(values.StrAmount), pg.strAssetType)

	// UTXO table view titles.
	pg.amountLabel = pg.generateLabel(name, pg.amountClickable)                                                 // Component 2
	pg.addressLabel = pg.generateLabel(values.String(values.StrAddress), pg.addressClickable)                   // Component 3
	pg.confirmationsLabel = pg.generateLabel(values.String(values.StrConfirmations), pg.confirmationsClickable) // component 4
	pg.dateLabel = pg.generateLabel(values.String(values.StrDateCreated), pg.dateClickable)                     // component 5

	// properties describes the spacing constants set for the display of UTXOs.
	pg.properties = []componentProperties{
		{direction: layout.Center, weight: 0.10}, // Component 1
		{direction: layout.W, weight: 0.20},      // Component 2
		{direction: layout.Center, weight: 0.25}, // Component 3
		{direction: layout.Center, weight: 0.20}, // Component 4
		{direction: layout.Center, weight: 0.25}, // Component 5
	}

	// clickables defines the event handlers mapped to individual title components.
	pg.clickables = []*cryptomaterial.Clickable{
		pg.amountClickable,        // Component 2
		pg.addressClickable,       // Component 3
		pg.confirmationsClickable, // Component 4
		pg.dateClickable,          // Component 5
	}

	pg.initializeFields()

	return pg
}

func (pg *ManualCoinSelectionPage) initializeFields() {
	pg.lastSortEvent = Lastclicked{clicked: -1}
	pg.selectedUTXOrows = make([]*sharedW.UnspentOutput, 0)
	pg.selectedAmount = 0

	pg.selectedUTXOs.Text = "0"
	pg.txSize.Text = pg.computeUTXOsSize()
	pg.totalAmount.Text = "0 " + pg.strAssetType
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ManualCoinSelectionPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	go func() {
		if err := pg.fetchAccountsInfo(); err != nil {
			log.Error(err)
		} else {
			// refresh the display to update the latest changes.
			pg.ParentWindow().Reload()
		}
	}()
}

func (pg *ManualCoinSelectionPage) fetchAccountsInfo() error {
	accounts, err := pg.WL.SelectedWallet.Wallet.GetAccountsRaw()
	if err != nil {
		return fmt.Errorf("querying the accounts names failed: %v", err)
	}

	pg.collapsibleList = make([]*cryptomaterial.Collapsible, 0, len(accounts.Accounts))
	pg.accountsUTXOs = make([]*AccountsUTXOInfo, 0, len(accounts.Accounts))
	for _, account := range accounts.Accounts {
		info, err := pg.WL.SelectedWallet.Wallet.UnspentOutputs(account.Number)
		if err != nil {
			return fmt.Errorf("querying the account (%v) info failed: %v", account.Number, err)
		}

		rowInfo := make([]*UTXOInfo, len(info))
		// create checkboxes for all the utxo available per accounts UTXOs.
		for i, row := range info {
			rowInfo[i] = &UTXOInfo{
				UnspentOutput: row,
				checkbox:      pg.Theme.CheckBox(new(widget.Bool), ""),
				addressCopy:   pg.Theme.NewClickable(false),
			}
		}

		pg.accountsUTXOs = append(pg.accountsUTXOs, &AccountsUTXOInfo{
			Details: rowInfo,
			Account: account.Name,
		})

		collapsible := pg.Theme.Collapsible()
		collapsible.IconPosition = cryptomaterial.Before
		collapsible.IconStyle = cryptomaterial.Caret
		pg.collapsibleList = append(pg.collapsibleList, collapsible)
	}

	return nil
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ManualCoinSelectionPage) HandleUserInteractions() {
	if pg.actionButton.Clicked() {
		sendPage := NewSendPage(pg.Load)
		sendPage.UpdateSelectedUTXOs(pg.selectedUTXOrows)
		pg.ParentNavigator().Display(sendPage)
	}

	if pg.fromCoinSelection.Clicked() {
		sendPage := NewSendPage(pg.Load)
		sendPage.UpdateSelectedUTXOs(pg.selectedUTXOrows)
		pg.ParentNavigator().Display(sendPage)
	}

	if pg.clearButton.Clicked() {
		for k := range pg.collapsibleList {
			for i := 0; i < len(pg.accountsUTXOs[k].Details); i++ {
				pg.accountsUTXOs[k].Details[i].checkbox.CheckBox = &widget.Bool{Value: false}
			}
		}
		pg.initializeFields()
	}

sortingLoop:
	for k, account := range pg.collapsibleList {
		if account.IsExpanded() {
		listLoop:
			for pos, component := range pg.clickables {
				if component == nil || !component.Clicked() {
					continue listLoop
				}

				if pg.sortingInProgress {
					break sortingLoop
				}

				pg.sortingInProgress = true

				if pos != pg.lastSortEvent.clicked {
					pg.lastSortEvent.clicked = pos
					pg.lastSortEvent.count = 0
				}
				pg.lastSortEvent.count++

				isAscendingOrder := pg.lastSortEvent.count%2 == 0
				sort.SliceStable(pg.accountsUTXOs[k].Details, func(i, j int) bool {
					return sortUTXOrows(i, j, pos, isAscendingOrder, pg.accountsUTXOs[k].Details)
				})

				pg.sortingInProgress = false
				break sortingLoop
			}
		}
	}

	// Update Summary information as the last section when handling events.
	for k := range pg.collapsibleList {
		for i := 0; i < len(pg.accountsUTXOs[k].Details); i++ {
			record := pg.accountsUTXOs[k].Details[i]
			if record.checkbox.CheckBox.Changed() {
				if record.checkbox.CheckBox.Value {
					pg.selectedUTXOrows = append(pg.selectedUTXOrows, record.UnspentOutput)
					pg.selectedAmount += record.Amount.ToCoin()
				} else {
					pg.selectedUTXOrows = pg.selectedUTXOrows[:len(pg.selectedUTXOrows)-1]
					pg.selectedAmount -= record.Amount.ToCoin()
				}

				pg.txSize.Text = pg.computeUTXOsSize()
				pg.selectedUTXOs.Text = fmt.Sprintf("%d", len(pg.selectedUTXOrows))
				pg.totalAmount.Text = fmt.Sprintf("%f %s", pg.selectedAmount, pg.strAssetType)
			}
		}
	}
}

func (pg *ManualCoinSelectionPage) computeUTXOsSize() string {
	wallet := pg.WL.SelectedWallet.Wallet

	switch wallet.GetAssetType() {
	case libutils.BTCWalletAsset:
		feeNSize, err := wallet.ComputeUTXOsSize(pg.selectedUTXOrows)
		if err != nil {
			log.Error(err)
		}
		return fmt.Sprintf("%d bytes", feeNSize)

	case libutils.DCRWalletAsset:
		return "--"

	default:
		return "--"
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *ManualCoinSelectionPage) OnNavigatedFrom() {
	pg.ctxCancel()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ManualCoinSelectionPage) Layout(gtx C) D {
	return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.MatchParent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.MatchParent,
					Height:      cryptomaterial.MatchParent,
					Orientation: layout.Vertical,
				}.Layout(gtx,
					layout.Rigid(pg.topSection),
					layout.Rigid(pg.summarySection),
					layout.Rigid(pg.accountListSection),
				)
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Inset{
					Top: values.MarginPadding5,
				}.Layout(gtx, func(gtx C) D {
					return layout.E.Layout(gtx, pg.actionButton.Layout)
				})
			}),
		)
	})
}

func (pg *ManualCoinSelectionPage) topSection(gtx C) D {
	return layout.Inset{Bottom: values.MarginPadding14}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.WrapContent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Horizontal,
				Alignment:   layout.Start,
				Clickable:   pg.fromCoinSelection,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Top:   values.MarginPadding8,
						Right: values.MarginPadding6,
					}.Layout(gtx, func(gtx C) D {
						return pg.Theme.Icons.ChevronLeft.LayoutSize(gtx, values.MarginPadding8)
					})
				}),
				layout.Rigid(pg.Theme.H6(values.String(values.StrSelectUTXO)).Layout),
			)
		})
	})
}

func (pg *ManualCoinSelectionPage) summarySection(gtx C) D {
	return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			topContainer := layout.UniformInset(values.MarginPadding15)
			return topContainer.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
							textLabel := pg.Theme.Label(values.TextSize16, values.String(values.StrSummary))
							textLabel.Font.Weight = text.SemiBold
							return textLabel.Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(0.22, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(pg.Theme.Label(values.TextSize14, values.String(values.StrSelectedUTXO)+": ").Layout),
									layout.Flexed(1, func(gtx C) D {
										return layout.W.Layout(gtx, pg.selectedUTXOs.Layout)
									}),
								)
							}),
							layout.Flexed(0.38, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(pg.Theme.Label(values.TextSize14, values.StringF(values.StrTxSize, " : ")).Layout),
									layout.Flexed(1, func(gtx C) D {
										return layout.W.Layout(gtx, pg.txSize.Layout)
									}),
								)
							}),
							layout.Flexed(0.4, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(pg.Theme.Label(values.TextSize14, values.String(values.StrTotalAmount)+": ").Layout),
									layout.Flexed(1, func(gtx C) D {
										return layout.W.Layout(gtx, pg.totalAmount.Layout)
									}),
								)
							}),
						)
					}),
				)
			})
		})
	})
}

func (pg *ManualCoinSelectionPage) accountListSection(gtx C) D {
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X

		return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					textLabel := pg.Theme.Label(values.TextSize16, values.String(values.StrAccountList))
					textLabel.Font.Weight = text.SemiBold
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(textLabel.Layout),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, pg.clearButton.Layout)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.Theme.List(pg.listContainer).Layout(gtx, len(pg.accountsUTXOs), func(gtx C, i int) D {
						return layout.Inset{
							Left:   values.MarginPadding5,
							Bottom: values.MarginPadding15,
						}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									collapsibleHeader := func(gtx C) D {
										t := pg.Theme.Label(values.TextSize16, pg.accountsUTXOs[i].Account)
										t.Font.Weight = text.SemiBold
										return t.Layout(gtx)
									}

									collapsibleBody := func(gtx C) D {
										if len(pg.accountsUTXOs[i].Details) == 0 {
											gtx.Constraints.Min.X = gtx.Constraints.Max.X
											return layout.Center.Layout(gtx,
												pg.Theme.Label(values.TextSize14, values.String(values.StrNoUTXOs)).Layout,
											)
										}
										return pg.accountListItemsSection(gtx, pg.accountsUTXOs[i].Details)
									}
									return pg.collapsibleList[i].Layout(gtx, collapsibleHeader, collapsibleBody)
								}),
							)
						})
					})
				}),
			)
		})
	})
}

func (pg *ManualCoinSelectionPage) generateLabel(txt interface{}, clickable *cryptomaterial.Clickable) labelCell {
	txtStr := ""
	switch n := txt.(type) {
	case string:
		txtStr = n
	case float64:
		txtStr = fmt.Sprintf("%0.4f", n) // to 4 decimal places
	case int32, int, int64:
		txtStr = fmt.Sprintf("%d", n)
	}

	lb := pg.Theme.Label(values.TextSize14, txtStr)
	if len(txtStr) > MaxAddressLen {
		// Only addresses have texts longer than 16 characters.
		lb.Text = txtStr[:MaxAddressLen] + "..."
		lb.Color = pg.Theme.Color.Primary
	}

	if clickable != nil {
		lb.Font.Weight = text.Bold
		lb.Color = pg.Theme.Color.Gray3
	}

	return labelCell{
		label:     lb,
		clickable: clickable,
	}
}

func (pg *ManualCoinSelectionPage) accountListItemsSection(gtx C, utxos []*UTXOInfo) D {
	return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return pg.rowItemsSection(gtx, nil, pg.amountLabel, pg.addressLabel, pg.confirmationsLabel, pg.dateLabel)
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X

				return pg.UTXOList.Layout(gtx, len(utxos), func(gtx C, index int) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
								v := utxos[index]
								checkButton := &v.checkbox                                                            // Component 1
								amountLabel := pg.generateLabel(v.Amount.ToCoin(), nil)                               // component 2
								addresslabel := pg.generateLabel(v.Address, nil)                                      // Component 3
								confirmationsLabel := pg.generateLabel(v.Confirmations, nil)                          // Component 4
								dateLabel := pg.generateLabel(libutils.FormatUTCShortTime(v.ReceiveTime.Unix()), nil) // Component 5

								// copy destination Address
								if v.addressCopy.Clicked() {
									clipboard.WriteOp{Text: v.Address}.Add(gtx.Ops)
									pg.Toast.Notify(values.String(values.StrTxHashCopied))
								}

								addressComponent := func(gtx C) D {
									return v.addressCopy.Layout(gtx, addresslabel.label.Layout)
								}
								return pg.rowItemsSection(gtx, checkButton, amountLabel, addressComponent, confirmationsLabel, dateLabel)
							})
						}),
						layout.Rigid(func(gtx C) D {
							// No divider for last row
							if index == len(utxos)-1 {
								return D{}
							}
							return pg.Theme.Separator().Layout(gtx)
						}),
					)
				})
			}),
		)
	})
}

func (pg *ManualCoinSelectionPage) rowItemsSection(gtx C, components ...interface{}) D {
	getRowItem := func(gtx C, index int) D {
		var widget layout.Widget
		c := components[index]

		switch n := c.(type) {
		case *cryptomaterial.CheckBoxStyle:
			widget = n.Layout
		case func(gtx C) D:
			widget = n
		case labelCell:
			pg.lastSortEvent.current = index - 1
			if n.clickable != nil {
				widget = func(gtx C) D {
					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.WrapContent,
						Height:      cryptomaterial.WrapContent,
						Orientation: layout.Horizontal,
						Alignment:   layout.Middle,
						Clickable:   n.clickable,
					}.Layout(gtx,
						layout.Rigid(n.label.Layout),
						layout.Rigid(func(gtx C) D {
							count := pg.lastSortEvent.count
							if pg.lastSortEvent.clicked == pg.lastSortEvent.current && count >= 0 {
								m := values.MarginPadding4
								inset := layout.Inset{Left: m}

								if count%2 == 0 { // add ascending icon
									inset.Bottom = m
									return inset.Layout(gtx, pg.Theme.Icons.CaretUp.Layout12dp)
								} // else add descending icon
								inset.Top = m
								return inset.Layout(gtx, pg.Theme.Icons.CaretDown.Layout12dp)
							}
							return D{}
						}),
					)
				}
			} else {
				widget = n.label.Layout
			}
		default:
			// create an empty default placeholder for unsupported widgets.
			widget = func(gtx C) D { return D{} }
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(widget))
	}

	max := float32(gtx.Constraints.Max.X)
	return pg.Theme.List(pg.utxosRow).Layout(gtx, len(components), func(gtx C, index int) D {
		c := pg.properties[index]
		gtx.Constraints.Min.X = int(max * c.weight)
		return c.direction.Layout(gtx, func(gtx C) D {
			return getRowItem(gtx, index)
		})
	})
}

func sortUTXOrows(i, j, pos int, ascendingOrder bool, elems []*UTXOInfo) bool {
	switch pos {
	case 0: // component 2 (Amount Component)
		if ascendingOrder {
			return elems[i].Amount.ToInt() > elems[j].Amount.ToInt()
		}
		return elems[i].Amount.ToInt() < elems[j].Amount.ToInt()
	case 1: // component 3 (Address Component)
		addresses := []string{elems[i].Address, elems[j].Address}
		if ascendingOrder {
			sort.Strings(addresses)
			return elems[i].Address == addresses[0]
		}
		sort.Sort(sort.Reverse(sort.StringSlice(addresses)))
		return elems[i].Address == addresses[0]
	case 2: // component 4 (Confirmations Component)
		if ascendingOrder {
			return elems[i].Confirmations > elems[j].Confirmations
		}
		return elems[i].Confirmations < elems[j].Confirmations
	case 3: // component 5 (Date Component)
		if ascendingOrder {
			return elems[i].ReceiveTime.Unix() > elems[j].ReceiveTime.Unix()
		}
		return elems[i].ReceiveTime.Unix() < elems[j].ReceiveTime.Unix()

	default:
		return false
	}
}
