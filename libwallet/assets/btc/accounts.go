package btc

import (
	"encoding/json"
	"math"
	"strconv"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/chaincfg"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

const (
	AddressGapLimit       uint32 = 20
	ImportedAccountNumber        = 2147483647
)

func (asset *BTCAsset) GetAccounts() (string, error) {
	accountsResponse, err := asset.GetAccountsRaw()
	if err != nil {
		return "", err
	}

	result, err := json.Marshal(accountsResponse)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func (asset *BTCAsset) GetAccountsRaw() (*sharedW.Accounts, error) {
	resp, err := asset.Internal().BTC.Accounts(asset.GetScope())
	if err != nil {
		return nil, err
	}

	accounts := make([]*sharedW.Account, len(resp.Accounts))
	for i, a := range resp.Accounts {
		balance, err := asset.GetAccountBalance(int32(a.AccountNumber))
		if err != nil {
			return nil, err
		}

		accounts[i] = &sharedW.Account{
			AccountProperties: sharedW.AccountProperties{
				AccountNumber:    a.AccountNumber,
				AccountName:      a.AccountName,
				ExternalKeyCount: a.ExternalKeyCount + AddressGapLimit, // Add gap limit
				InternalKeyCount: a.InternalKeyCount + AddressGapLimit,
				ImportedKeyCount: a.ImportedKeyCount,
			},
			WalletID: asset.ID,
			Balance:  balance,
		}
	}

	return &sharedW.Accounts{
		CurrentBlockHash:   resp.CurrentBlockHash[:],
		CurrentBlockHeight: resp.CurrentBlockHeight,
		Accounts:           accounts,
	}, nil
}

func (asset *BTCAsset) GetAccount(accountNumber int32) (*sharedW.Account, error) {
	accounts, err := asset.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	for _, account := range accounts.Accounts {
		if account.AccountNumber == uint32(accountNumber) {
			return account, nil
		}
	}

	return nil, errors.New(utils.ErrNotExist)
}

func (asset *BTCAsset) GetAccountBalance(accountNumber int32) (*sharedW.Balance, error) {
	balance, err := asset.Internal().BTC.CalculateAccountBalances(uint32(accountNumber), asset.RequiredConfirmations())
	if err != nil {
		return nil, err
	}

	return &sharedW.Balance{
		Total:          BTCAmount(balance.Total),
		Spendable:      BTCAmount(balance.Spendable),
		ImmatureReward: BTCAmount(balance.ImmatureReward),
	}, nil
}

func (asset *BTCAsset) SpendableForAccount(account int32) (int64, error) {
	bals, err := asset.Internal().BTC.CalculateAccountBalances(uint32(account), asset.RequiredConfirmations())
	if err != nil {
		return 0, utils.TranslateError(err)
	}
	return int64(bals.Spendable), nil
}

func (asset *BTCAsset) UnspentOutputs(account int32) ([]*ListUnspentResult, error) {
	accountName, err := asset.AccountName(account)
	if err != nil {
		return nil, err
	}

	unspents, err := asset.Internal().BTC.ListUnspent(0, math.MaxInt32, accountName)
	if err != nil {
		return nil, err
	}
	resp := make([]*ListUnspentResult, 0, len(unspents))

	for _, utxo := range unspents {
		resp = append(resp, &ListUnspentResult{
			TxID:          utxo.TxID,
			Vout:          utxo.Vout,
			Address:       utxo.Address,
			ScriptPubKey:  utxo.ScriptPubKey,
			RedeemScript:  utxo.RedeemScript,
			Amount:        utxo.Amount,
			Confirmations: int64(utxo.Confirmations),
			Spendable:     utxo.Spendable,
		})
	}

	return resp, nil
}

func (asset *BTCAsset) CreateNewAccount(accountName, privPass string) (int32, error) {
	err := asset.UnlockWallet(privPass)
	if err != nil {
		return -1, err
	}

	defer asset.LockWallet()

	return asset.NextAccount(accountName)
}

func (asset *BTCAsset) NextAccount(accountName string) (int32, error) {

	if asset.IsLocked() {
		return -1, errors.New(utils.ErrWalletLocked)
	}

	accountNumber, err := asset.Internal().BTC.NextAccount(asset.GetScope(), accountName)
	if err != nil {
		return -1, err
	}

	return int32(accountNumber), nil
}

func (asset *BTCAsset) RenameAccount(accountNumber int32, newName string) error {
	err := asset.Internal().BTC.RenameAccount(asset.GetScope(), uint32(accountNumber), newName)
	if err != nil {
		return utils.TranslateError(err)
	}

	return nil
}

func (asset *BTCAsset) AccountName(accountNumber int32) (string, error) {
	name, err := asset.AccountNameRaw(uint32(accountNumber))
	if err != nil {
		return "", utils.TranslateError(err)
	}
	return name, nil
}

func (asset *BTCAsset) AccountNameRaw(accountNumber uint32) (string, error) {
	return asset.Internal().BTC.AccountName(asset.GetScope(), accountNumber)
}

func (asset *BTCAsset) AccountNumber(accountName string) (int32, error) {
	accountNumber, err := asset.Internal().BTC.AccountNumber(asset.GetScope(), accountName)
	return int32(accountNumber), utils.TranslateError(err)
}

func (asset *BTCAsset) HasAccount(accountName string) bool {
	_, err := asset.Internal().BTC.AccountNumber(asset.GetScope(), accountName)
	return err == nil
}

func (asset *BTCAsset) HDPathForAccount(accountNumber int32) (string, error) {
	var hdPath string
	if asset.chainParams.Name == chaincfg.MainNetParams.Name {
		hdPath = MainnetHDPath
	} else {
		hdPath = TestnetHDPath
	}

	return hdPath + strconv.Itoa(int(accountNumber)), nil
}
