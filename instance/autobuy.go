package instance

import (
	"fmt"
	"github.com/BridgeSenseDev/Dank-Memer-Grinder/gateway"
	"regexp"
	"strconv"
	"strings"

	"github.com/BridgeSenseDev/Dank-Memer-Grinder/discord/types"
)

type AutoBuyState struct {
	shopTypeIndex int
	count         int
	itemEmojiName string
	price         int
}

var globalAutoBuyState = AutoBuyState{
	shopTypeIndex: 0,
	count:         0,
	itemEmojiName: "",
	price:         0,
}

func (in *Instance) setAutoBuyState(shopTypeIndex, count int, itemEmojiName string, price int) {
	globalAutoBuyState.shopTypeIndex = shopTypeIndex
	globalAutoBuyState.count = count
	globalAutoBuyState.itemEmojiName = itemEmojiName
	globalAutoBuyState.price = price
}

func (in *Instance) findAndClickButton(message gateway.EventMessage, targetEmojiName string) bool {
	for rowIndex, component := range message.Components {
		if rowIndex == 0 || rowIndex == 3 {
			continue
		}
		for columnIndex, button := range component.(*types.ActionsRow).Components {
			if button.(*types.Button).Emoji.Name == targetEmojiName {
				err := in.ClickButton(message, rowIndex, columnIndex)
				if err != nil {
					in.Log("discord", "ERR", fmt.Sprintf("Failed to click autobuy button: %s", err.Error()))
				}
				in.Log("others", "INF", "Done clicking button.")
				return true
			}
		}
	}

	return false
}

func (in *Instance) shopBuy(shopMsg gateway.EventMessage) {
	shopTypeOptions := shopMsg.Components[0].(*types.ActionsRow).Components[0].(*types.SelectMenu).Options
	if !shopTypeOptions[globalAutoBuyState.shopTypeIndex].Default {
		err := in.ChooseSelectMenu(shopMsg, 0, 0, []string{shopTypeOptions[globalAutoBuyState.shopTypeIndex].Value})
		if err != nil {
			in.Log("discord", "ERR", fmt.Sprintf("Failed to choose shop view select menu: %s", err.Error()))
		}
	} else {
		if !in.findAndClickButton(shopMsg, globalAutoBuyState.itemEmojiName) {
			err := in.ClickButton(shopMsg, 3, 1)
			if err != nil {
				in.Log("discord", "ERR", fmt.Sprintf("Failed to click next autobuy page button: %s", err.Error()))
			}
			in.Log("others", "INF", "clicked next button")
		}
	}
}

func (in *Instance) AutoBuyMessageUpdate(message gateway.EventMessage) {
	embed := message.Embeds[0]

	if embed.Title == "Dank Memer Shop" && globalAutoBuyState.itemEmojiName != "" {
		if strings.Contains(embed.Footer.Text, "Page 1") {
			in.Log("others", "ERR", "Failed to find autobuy button")
			//in.setAutoBuyState(0, 0, "", 0)
			//in.UnpauseCommands()
			/* check this part, instance/instance.go 200-ish line*/
			return
		}

		in.shopBuy(message)
	}
}

func (in *Instance) AutoBuyMessageCreate(message gateway.EventMessage) {
	embed := message.Embeds[0]
	if strings.Contains(embed.Description, "You don't have a shovel") && in.Cfg.AutoBuy.Shovel.State {
		in.setAutoBuyState(0, 1, "IronShovel", 50000)
		in.Log("others", "INF", "Attempting to buy")
	} else if strings.Contains(embed.Description, "You don't have a hunting rifle") && in.Cfg.AutoBuy.HuntingRifle.State {
		in.setAutoBuyState(0, 1, "LowRifle", 50000)
	} else if embed.Title == "Your lifesaver protected you!" && in.Cfg.AutoBuy.LifeSavers.State {
		re := regexp.MustCompile(`You have (\d+) Life Saver left`)
		match := re.FindStringSubmatch(message.Components[0].(*types.ActionsRow).Components[0].(*types.Button).Label)

		if len(match) > 1 {
			remaining, err := strconv.Atoi(match[1])
			if err != nil {
				in.Log("important", "ERR", fmt.Sprintf("Failed to determine amount of lifesavers required: %s", err.Error()))
			}

			required := in.Cfg.AutoBuy.LifeSavers.Amount

			if remaining < required {
				in.setAutoBuyState(0, required-remaining, "LifeSaver", (required-remaining)*250000)
			}
		} else {
			in.Log("important", "ERR", "Failed to determine amount of lifesavers required")
		}
	} else if embed.Title == "You died!" {
		in.setAutoBuyState(0, in.Cfg.AutoBuy.LifeSavers.Amount, "LifeSaver", in.Cfg.AutoBuy.LifeSavers.Amount*250000)
	} else if embed.Title == "Pending Confirmation" {
		if strings.Contains(embed.Description, "Would you like to use your **<:Coupon:977969734307971132> Shop Coupon**") {
			err := in.ClickButton(message, 0, 0)
			if err != nil {
				in.Log("important", "ERR", "Failed to click decline shop coupon button")
			}
		} else if strings.Contains(embed.Description, "Are you sure you want to buy") {
			err := in.ClickButton(message, 0, 1)
			if err != nil {
				in.Log("important", "ERR", "Failed to click shop buy confirmation button")
			}
		}
		return
	} else if message.Embeds[0].Title == "Dank Memer Shop" && globalAutoBuyState.itemEmojiName != "" {
		in.shopBuy(message)
		return
	} else {
		return
	}

	if globalAutoBuyState.itemEmojiName == "" {
		return
	}

	in.PauseCommands(false)
	in.Log("others", "INF", fmt.Sprintf("Auto buying %s", globalAutoBuyState.itemEmojiName))

	err := in.SendCommand("withdraw", map[string]string{"amount": strconv.Itoa(globalAutoBuyState.price)}, true)
	if err != nil {
		in.Log("discord", "ERR", fmt.Sprintf("Failed to send autobuy /withdraw command: %s", err.Error()))
	}

	err = in.SendSubCommand("shop", "view", nil, true)
	if err != nil {
		in.Log("discord", "ERR", fmt.Sprintf("Failed to send /shop view command: %s", err.Error()))
	}
}

func (in *Instance) AutoBuyModalCreate(modal gateway.EventModalCreate) {
	if modal.Title == "Dank Memer Shop" {
		in.Log("others", "INF", "Modal detected")
		in.Log("others", "INF", fmt.Sprintf("Amount to buy: %s" ,strconv.Itoa(globalAutoBuyState.count)))
		if globalAutoBuyState.count != 0 {
			modal.Components[0].(*types.ActionsRow).Components[0].(*types.TextInput).Value = strconv.Itoa(globalAutoBuyState.count)
			err := in.SubmitModal(modal)
			if err != nil {
				in.Log("discord", "ERR", fmt.Sprintf("Failed to submit autobuy modal: %s", err.Error()))
			}
			in.Log("others", "INF", fmt.Sprintf("Auto bought %s", globalAutoBuyState.itemEmojiName))
		} else {
			in.Log("others", "INF", "spam detected....")
		}
		in.setAutoBuyState(0, 0, "", 0)
		in.UnpauseCommands()
	}
}
