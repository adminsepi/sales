package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	bot             *tgbotapi.BotAPI
	uploadFolder    = "uploads"
	signedFolder    = "signed"
	signQueue       = make([]signRequest, 0)
	mutex           sync.Mutex
	averageSignTime = 30 * time.Second
)

type signRequest struct {
	userID    int64
	chatID    int64
	fileName  string
	fileID    string
}

type Channel struct {
	Name   string
	URL    string
	ChatID int64
}

var channels = []Channel{
	{Name: "کانال پشتیبانی #سالس_استرول", URL: "https://t.me/salesestrol", ChatID: -1002721560354},
	{Name: "کانال اختصاصی■VIP■", URL: "https://t.me/+XgPHewjiAdc1ZmI8", ChatID: -1002337225404},
	{Name: "گروه چت و مشورت🔞", URL: "https://t.me/+EKFD_UpMaEpjODc0", ChatID: -1002778968668},
}

func init() {
	if _, err := os.Stat(uploadFolder); os.IsNotExist(err) {
		os.Mkdir(uploadFolder, 0755)
	}
	if _, err := os.Stat(signedFolder); os.IsNotExist(err) {
		os.Mkdir(signedFolder, 0755)
	}
}

func isRealMember(userID int64) bool {
	for _, channel := range channels {
		_, err := bot.GetChatMember(tgbotapi.ChatConfigWithUser{
			ChatID: channel.ChatID,
			UserID: userID,
		})
		if err != nil {
			return false
		}
	}
	return true
}

func signApk(inputPath, outputPath string) error {
	cmd := exec.Command("jarsigner", "-keystore", os.Getenv("KEYSTORE_PATH"),
		"-storepass", os.Getenv("KEYSTORE_PASSWORD"), "-keypass", os.Getenv("KEY_PASSWORD"),
		"-signedjar", outputPath, inputPath, os.Getenv("KEY_ALIAS"))
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("Error signing APK: %s", stderr.String())
		return fmt.Errorf("خطا در امضا: %s", stderr.String())
	}
	return nil
}

func sendMessage(chatID int64, text string, buttons [][]tgbotapi.InlineKeyboardButton) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	if len(buttons) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	}
	_, err := bot.Send(msg)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("خطا در ارسال پیام به %d: %v", chatID, err)))
	}
}

func sendFile(chatID int64, filePath, caption string) {
	msg := tgbotapi.NewDocumentUpload(chatID, filePath)
	msg.Caption = caption
	msg.ParseMode = "HTML"
	_, err := bot.Send(msg)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("خطا در ارسال فایل به %d: %v", chatID, err)))
	}
}

func main() {
	var err error
	bot, err = tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	updates := bot.GetUpdatesChan(tgbotapi.UpdateConfig{Timeout: 60})

	go func() {
		for update := range updates {
			if update.Message != nil {
				chatID := update.Message.Chat.ID
				userID := update.Message.From.ID
				text := update.Message.Text

				if text == "/start" {
					joinButtons := make([][]tgbotapi.InlineKeyboardButton, 0)
					for _, ch := range channels {
						joinButtons = append(joinButtons, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonURL("عضویت در "+ch.Name, ch.URL)))
					}
					joinButtons = append(joinButtons, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("تایید عضویت ✅", "verify_me")))
					sendMessage(chatID, `💥 پیام ادمین <b>#سالس_استرول</b>: 💥
🆔️ PV SUPPORTER: <b>@RealSalesestrol</b>
🔐 برای امضای فایل APK، لطفاً در کانال‌ها و گروه زیر عضو شوید:`, joinButtons)
				} else if text == "/sign" {
					if isRealMember(userID) {
						sendMessage(chatID, `🖋 لطفاً فایل APK خود را آپلود کنید.
امضا توسط <b>#سالس_استرول</b> با طرح‌های v2 و v3 انجام خواهد شد.`)
					} else {
						failed := make([]string, 0)
						for _, ch := range channels {
							if !isRealMember(userID) {
								failed = append(failed, ch.Name)
							}
						}
						sendMessage(chatID, fmt.Sprintf(`⚠️ شما هنوز در موارد زیر عضو نشده‌اید:
%s

لطفاً ابتدا در کانال‌ها و گروه <b>#سالس_استرول</b> عضو شوید و دوباره تلاش کنید.`, strings.Join(failed, ", ")), [][]tgbotapi.InlineKeyboardButton{
							{tgbotapi.NewInlineKeyboardButtonURL("عضویت", channels[0].URL)},
							{tgbotapi.NewInlineKeyboardButtonData("تلاش مجدد", "verify_me")},
						})
					}
				} else if update.Message.Document != nil {
					file := update.Message.Document
					fileName := file.FileName
					mimeType := file.MimeType
					fileSize := float64(file.FileSize) / 1024 // KB

					if !strings.HasSuffix(strings.ToLower(fileName), ".apk") && !strings.Contains(mimeType, "apk") {
						sendMessage(chatID, fmt.Sprintf("⚠️ فایل %s یک APK معتبر نیست! لطفاً فایل APK آپلود کنید.", fileName))
						continue
					}

					if fileSize > 50*1024 {
						sendMessage(chatID, "⚠️ فایل APK خیلی بزرگه! حداکثر حجم مجاز 50 مگابایته.")
						continue
					}

					if !isRealMember(userID) {
						failed := make([]string, 0)
						for _, ch := range channels {
							if !isRealMember(userID) {
								failed = append(failed, ch.Name)
							}
						}
						sendMessage(chatID, fmt.Sprintf(`⚠️ شما هنوز در موارد زیر عضو نشده‌اید:
%s

لطفاً ابتدا در کانال‌ها و گروه <b>#سالس_استرول</b> عضو شوید.`, strings.Join(failed, ", ")), [][]tgbotapi.InlineKeyboardButton{
							{tgbotapi.NewInlineKeyboardButtonURL("عضویت", channels[0].URL)},
							{tgbotapi.NewInlineKeyboardButtonData("تلاش مجدد", "verify_me")},
						})
						continue
					}

					fileID := file.FileID
					fileURL, err := bot.GetFileDirectURL(fileID)
					if err != nil {
						sendMessage(chatID, "⚠️ خطا در دریافت فایل از تلگرام!")
						bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("خطا در دریافت فایل %s: %v", fileName, err)))
						continue
					}

					inputPath := filepath.Join(uploadFolder, fileName)
					outputPath := filepath.Join(signedFolder, "signed_"+fileName)

					resp, err := http.Get(fileURL)
					if err != nil {
						sendMessage(chatID, "⚠️ خطا در دانلود فایل!")
						bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("خطا در دانلود %s: %v", fileName, err)))
						continue
					}
					defer resp.Body.Close()

					out, err := os.Create(inputPath)
					if err != nil {
						sendMessage(chatID, "⚠️ خطا در ذخیره فایل!")
						bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("خطا در ذخیره %s: %v", fileName, err)))
						continue
					}
					defer out.Close()

					_, err = io.Copy(out, resp.Body)
					if err != nil {
						sendMessage(chatID, "⚠️ خطا در کپی فایل!")
						bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("خطا در کپی %s: %v", fileName, err)))
						continue
					}

					mutex.Lock()
					signQueue = append(signQueue, signRequest{userID: userID, chatID: chatID, fileName: fileName, fileID: fileID})
					queuePos := len(signQueue)
					estTime := time.Duration(queuePos) * averageSignTime / time.Minute
					mutex.Unlock()

					sendMessage(chatID, fmt.Sprintf(`✅ فایل APK شما (%s) دریافت شد!
موقعیت شما در صف: %d
تخمین زمان امضا: حدود %d دقیقه
لطفاً صبر کنید...`, fileName, queuePos, estTime), nil)

					if queuePos == 1 {
						go func() {
							for len(signQueue) > 0 {
								mutex.Lock()
								req := signQueue[0]
								mutex.Unlock()

								inputPath := filepath.Join(uploadFolder, req.fileName)
								outputPath := filepath.Join(signedFolder, "signed_"+req.fileName)

								err := signApk(inputPath, outputPath)
								if err == nil {
									sendFile(req.chatID, outputPath, `✅ فایل APK شما با موفقیت امضا شد (v2+v3، سازگار با اندروید 7.0+)!
امضا توسط <b>#سالس_استرول</b> | <b>@RealSalesestrol</b>`)
									os.Remove(inputPath)
									os.Remove(outputPath)
								} else {
									sendMessage(req.chatID, fmt.Sprintf("❌ خطا در امضای فایل: %v", err))
									bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("خطا در امضای %s برای کاربر %d: %v", req.fileName, req.userID, err)))
									os.Remove(inputPath)
								}

								mutex.Lock()
								signQueue = signQueue[1:]
								mutex.Unlock()
							}
						}()
					}
				}
			} else if update.CallbackQuery != nil {
				if update.CallbackQuery.Data == "verify_me" {
					if isRealMember(update.CallbackQuery.From.ID) {
						sendMessage(update.CallbackQuery.Message.Chat.ID, `🎉 عضویت شما تأیید شد!
برای امضای فایل APK، از دستور /sign استفاده کنید و سپس فایل APK خود را آپلود کنید.
مدیریت: <b>#سالس_استرول</b> | <b>@RealSalesestrol</b>`, nil)
					} else {
						failed := make([]string, 0)
						for _, ch := range channels {
							if !isRealMember(update.CallbackQuery.From.ID) {
								failed = append(failed, ch.Name)
							}
						}
						sendMessage(update.CallbackQuery.Message.Chat.ID, fmt.Sprintf(`⚠️ شما هنوز در موارد زیر عضو نشده‌اید:
%s

لطفاً ابتدا در کانال‌ها و گروه <b>#سالس_استرول</b> عضو شوید و دوباره تلاش کنید.`, strings.Join(failed, ", ")), [][]tgbotapi.InlineKeyboardButton{
							{tgbotapi.NewInlineKeyboardButtonURL("عضویت", channels[0].URL)},
							{tgbotapi.NewInlineKeyboardButtonData("تلاش مجدد", "verify_me")},
						})
					}
					bot.AnswerCallbackQuery(tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID})
				}
			}
		}
	}()

	select {} // نگه داشتن برنامه فعال
}