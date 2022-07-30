package privacy

import (
	"context"

	"gioui.org/layout"

	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/listeners"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/values"
	"gitlab.com/raedah/cryptopower/wallet"
)

const AccountMixerPageID = "AccountMixer"

type AccountMixerPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	*listeners.AccountMixerNotificationListener

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	wallet *dcrlibwallet.Wallet

	toggleMixer *decredmaterial.Switch

	mixerCompleted bool

	totalBalance  dcrutil.Amount
	mixerProgress decredmaterial.ProgressBarStyle
}

func NewAccountMixerPage(l *load.Load) *AccountMixerPage {
	pg := &AccountMixerPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(AccountMixerPageID),
		wallet:           wallet,
		toggleMixer:      l.Theme.Switch(),
		mixerProgress:    l.Theme.ProgressBar(0),
	}
	pg.mixerProgress.Height = values.MarginPadding18
	pg.mixerProgress.Radius = decredmaterial.Radius(2)
	totalBalance, _ := components.CalculateTotalWalletsBalance(pg.Load)
	pg.totalBalance = totalBalance.Total

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *AccountMixerPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.listenForMixerNotifications()
	pg.toggleMixer.SetChecked(pg.WL.SelectedWallet.Wallet.IsAccountMixerActive())
}

func (pg *AccountMixerPage) MixerInfoLayout(gtx C, l *load.Load, mixerActive bool, button layout.Widget, mixerInfo layout.Widget) D {
	mixedBalance := "0.00"
	unmixedBalance := "0.00"
	accounts, _ := pg.wallet.GetAccountsRaw()

	for _, acct := range accounts.Acc {
		if acct.Number == pg.wallet.MixedAccountNumber() {
			mixedBalance = dcrutil.Amount(acct.TotalBalance).String()
		} else if acct.Number == pg.wallet.UnmixedAccountNumber() {
			unmixedBalance = dcrutil.Amount(acct.TotalBalance).String()
		}
	}
	return l.Theme.Card().Layout(gtx, func(gtx C) D {
		txt := l.Theme.H6(values.String(values.StrBalance))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.UniformInset(values.MarginPadding40).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Bottom: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(txt.Layout),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{Left: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
											return components.LayoutBalance(gtx, pg.Load, pg.totalBalance.String())
										})
									}),
									layout.Flexed(1, func(gtx C) D {
										return layout.E.Layout(gtx, func(gtx C) D {
											txt := l.Theme.H6("Mix")
											return layout.Flex{}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
														return txt.Layout(gtx)
													})
												}),
												layout.Rigid(button),
											)
										})
									}),
								)
							})
						}),
						layout.Rigid(func(gtx C) D {
							return l.Theme.Separator().Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										txt := l.Theme.H6(values.String(values.StrMixer))
										return txt.Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{Left: values.MarginPadding20, Right: values.MarginPadding40}.Layout(gtx, func(gtx C) D {
											return pg.mixerProgress.Layout(gtx)
										})
									}),
								)
							})
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Right: values.MarginPadding15, Left: values.MarginPadding15, Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							ic := l.Theme.Icons.Mixer
							return ic.Layout12dp(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							txt := l.Theme.H6(values.String(values.StrMixed))
							return layout.Inset{Left: values.MarginPadding11}.Layout(gtx, func(gtx C) D {
								return txt.Layout(gtx)
							})
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.Inset{Right: values.MarginPadding25}.Layout(gtx, func(gtx C) D {
								return layout.E.Layout(gtx, func(gtx C) D {
									return components.LayoutBalance(gtx, pg.Load, mixedBalance)
								})
							})
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Right: values.MarginPadding40, Left: values.MarginPadding40}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Max.X = gtx.Constraints.Max.X/2 - 20
							return layout.W.Layout(gtx, l.Theme.Separator().Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: values.MarginPadding10, Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
								ic := l.Theme.Icons.Mixer
								return layout.Center.Layout(gtx, func(gtx C) D {
									return ic.Layout36dp(gtx)
								})
							})
						}),
						layout.Rigid(func(gtx C) D {
							return layout.E.Layout(gtx, l.Theme.Separator().Layout)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Right: values.MarginPadding15, Left: values.MarginPadding15, Bottom: values.MarginPadding40}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							ic := l.Theme.Icons.Mixer
							return ic.Layout12dp(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							txt := l.Theme.H6(values.String(values.StrUnmixed))
							return layout.Inset{Left: values.MarginPadding11}.Layout(gtx, func(gtx C) D {
								return txt.Layout(gtx)
							})
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.Inset{Right: values.MarginPadding25}.Layout(gtx, func(gtx C) D {
								return layout.E.Layout(gtx, func(gtx C) D {
									return components.LayoutBalance(gtx, pg.Load, unmixedBalance)
								})
							})
						}),
					)
				})
			}),
		)
	})
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *AccountMixerPage) Layout(gtx layout.Context) layout.Dimensions {
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *AccountMixerPage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	return components.UniformPadding(gtx, func(gtx C) D {
		return pg.MixerInfoLayout(gtx, pg.Load, pg.wallet.IsAccountMixerActive(), pg.toggleMixer.Layout, func(gtx C) D {
			mixedBalance := "0.00"
			unmixedBalance := "0.00"
			accounts, _ := pg.wallet.GetAccountsRaw()

			for _, acct := range accounts.Acc {
				if acct.Number == pg.wallet.MixedAccountNumber() {
					mixedBalance = dcrutil.Amount(acct.TotalBalance).String()
				} else if acct.Number == pg.wallet.UnmixedAccountNumber() {
					unmixedBalance = dcrutil.Amount(acct.TotalBalance).String()
				}
			}

			return components.MixerInfoContentWrapper(gtx, pg.Load, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								txt := pg.Theme.Label(values.TextSize14, "Unmixed balance")
								txt.Color = pg.Theme.Color.GrayText2
								return txt.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								return components.LayoutBalance(gtx, pg.Load, unmixedBalance)
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Center.Layout(gtx, pg.Theme.Icons.ArrowDownIcon.Layout24dp)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								t := pg.Theme.Label(values.TextSize14, "Mixed balance")
								t.Color = pg.Theme.Color.GrayText2
								return t.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								return components.LayoutBalance(gtx, pg.Load, mixedBalance)
							}),
						)
					}),
				)
			})
		})
	})
}

func (pg *AccountMixerPage) layoutMobile(gtx layout.Context) layout.Dimensions {
	return D{}
}

/*
func (pg *AccountMixerPage) shufflePortForCurrentNet() string {
	if pg.WL.Wallet.Net == libwallet.Testnet3 {
		return libwallet.TestnetShufflePort
	}

	return libwallet.MainnetShufflePort
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *AccountMixerPage) HandleUserInteractions() {
	if pg.toggleMixer.Changed() {
		if pg.toggleMixer.IsChecked() {
			go pg.showModalPasswordStartAccountMixer()
		} else {
			pg.toggleMixer.SetChecked(true)
			info := modal.NewCustomModal(pg.Load).
				Title("Cancel mixer?").
				Body("Are you sure you want to cancel mixer action?").
				SetNegativeButtonText(values.String(values.StrNo)).
				SetPositiveButtonText(values.String(values.StrYes)).
				SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
					pg.toggleMixer.SetChecked(false)
					go pg.WL.MultiWallet.StopAccountMixer(pg.WL.SelectedWallet.Wallet.ID)
					return true
				})
			pg.ParentWindow().ShowModal(info)
		}
	}

	if pg.mixerCompleted {
		pg.toggleMixer.SetChecked(false)
		pg.mixerCompleted = false
		pg.ParentWindow().Reload()
	}

	if pg.backButton.Button.Clicked() {
		pg.ParentNavigator().ClosePagesAfter(components.WalletsPageID)
	}
}

func (pg *AccountMixerPage) showModalPasswordStartAccountMixer() {
	passwordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title("Confirm to mix account").
		SetNegativeButtonCallback(func() {
			pg.toggleMixer.SetChecked(false)
		}).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := pg.WL.MultiWallet.StartAccountMixer(pg.WL.SelectedWallet.Wallet.ID, password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			pm.Dismiss()
			return false
		})
	pg.ParentWindow().ShowModal(passwordModal)
}

func (pg *AccountMixerPage) listenForMixerNotifications() {
	if pg.AccountMixerNotificationListener != nil {
		return
	}

	pg.AccountMixerNotificationListener = listeners.NewAccountMixerNotificationListener()
	err := pg.WL.MultiWallet.AddAccountMixerNotificationListener(pg, AccountMixerPageID)
	if err != nil {
		log.Errorf("Error adding account mixer notification listener: %+v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-pg.MixerChan:
				if n.RunStatus == wallet.MixerStarted {
					pg.Toast.Notify("Mixer start Successfully")
					pg.ParentWindow().Reload()
				}

				if n.RunStatus == wallet.MixerEnded {
					pg.mixerCompleted = true
					pg.ParentWindow().Reload()
				}

			case <-pg.ctx.Done():
				pg.WL.MultiWallet.RemoveAccountMixerNotificationListener(AccountMixerPageID)
				close(pg.MixerChan)
				pg.AccountMixerNotificationListener = nil
				return
			}
		}
	}()
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *AccountMixerPage) OnNavigatedFrom() {
	pg.ctxCancel()
}
