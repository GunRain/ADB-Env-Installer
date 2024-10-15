package main

import (
	"archive/zip"
	_ "embed"
	"image/color"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sys/windows/registry"
)

const (
	targetWidth    = 500
	targetHeight   = 300
	targetTextSize = 21

	targetEnvDir = ".adb-env"
)

//go:embed font.ttf
var fontData []byte

//go:embed icon.png
var iconData []byte

type App_Theme struct{}

var _ fyne.Theme = (*App_Theme)(nil)

func (m *App_Theme) Font(style fyne.TextStyle) fyne.Resource {
	return &fyne.StaticResource{
		StaticName:    "font.ttf",
		StaticContent: fontData,
	}
}

func (m *App_Theme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (m *App_Theme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *App_Theme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func cannot(reson string) *fyne.Container {
	txt := canvas.NewText("安装失败", theme.ForegroundColor())
	txt.Alignment = fyne.TextAlignCenter
	txt.TextSize = targetTextSize / 3 * 4
	button := widget.NewButton("退出", func() { os.Exit(-1) })

	content := container.NewCenter(
		container.NewVBox(
			txt,
			widget.NewLabel(""),
			button,
		),
	)
	return content
}

func main() {
	Env_Installer := app.New()
	Env_Installer.SetIcon(fyne.NewStaticResource("icon.png", iconData))
	window := Env_Installer.NewWindow("ADB与Fastboot环境安装器")
	Theme := &App_Theme{}
	Env_Installer.Settings().SetTheme(Theme)

	txt := canvas.NewText("为当前用户安装ADB与Fastboot环境", theme.ForegroundColor())
	txt.Alignment = fyne.TextAlignCenter
	txt.TextSize = targetTextSize
	txt2 := canvas.NewText("将会通过网络下载最新版平台工具", theme.ForegroundColor())
	txt2.Alignment = fyne.TextAlignCenter
	txt2.TextSize = targetTextSize
	txt3 := canvas.NewText("由哔哩哔哩@泠熙子殿下制作，感谢您的使用", theme.ForegroundColor())
	txt3.Alignment = fyne.TextAlignCenter
	txt3.TextSize = targetTextSize / 3 * 2
	button := widget.NewButton("进行安装", func() {
		txt0 := canvas.NewText("正在安装中，请耐心等待", theme.ForegroundColor())
		txt0.Alignment = fyne.TextAlignCenter
		txt0.TextSize = targetTextSize / 3 * 4
		content0 := container.NewCenter(
			container.NewVBox(
				txt0,
			),
		)
		window.SetContent(content0)

		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			window.SetContent(cannot("无法获取变量 USERPROFILE 值"))
			return
		}
		targetDir := filepath.Join(userProfile, targetEnvDir)
		_, err := os.Stat(targetDir)
		if err == nil {
			err = os.RemoveAll(targetDir)
			if err != nil {
				window.SetContent(cannot("无法删除文件夹 " + targetDir + " : " + err.Error()))
				return
			}
		}
		err = os.Mkdir(targetDir, os.FileMode(0777))
		if err != nil {
			window.SetContent(cannot("无法创建文件夹 " + targetDir + " : " + err.Error()))
			return
		}
		zipFile := filepath.Join(targetDir, "platform-tools.zip")
		out, err := os.Create(zipFile)
		if err != nil {
			window.SetContent(cannot("无法创建文件 " + zipFile + " : " + err.Error()))
			return
		}

		resp, err := http.Get("https://googledownloads.cn/android/repository/platform-tools-latest-windows.zip")
		if err != nil {
			window.SetContent(cannot("无法下载文件: " + err.Error()))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			window.SetContent(cannot("服务器返回非 200 状态码: " + strconv.Itoa(resp.StatusCode)))
			return
		}
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			window.SetContent(cannot("无法储存文件: " + err.Error()))
			return
		}
		out.Close()

		r, err := zip.OpenReader(zipFile)
		if err != nil {
			window.SetContent(cannot("无法打开文件: " + err.Error()))
			return
		}
		var rootPrefix string
		if len(r.File) > 0 {
			rootPrefix = strings.Split(r.File[0].Name, "/")[0] + "/"
		}
		for _, f := range r.File {
			fpath := filepath.Join(targetDir, strings.TrimPrefix(f.Name, rootPrefix))
			if f.FileInfo().IsDir() {
				os.MkdirAll(fpath, os.ModePerm)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				window.SetContent(cannot("无法创建文件夹 " + filepath.Dir(fpath) + " : " + err.Error()))
				return
			}
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				window.SetContent(cannot("无法创建文件 " + fpath + " : " + err.Error()))
				return
			}
			rc, err := f.Open()
			if err != nil {
				window.SetContent(cannot("无法打开文件: " + err.Error()))
				return
			}
			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()
			if err != nil {
				window.SetContent(cannot("无法解压文件: " + err.Error()))
				return
			}
		}
		r.Close()
		if err := os.Remove(zipFile); err != nil {
			window.SetContent(cannot("无法删除文件 " + zipFile + " : " + err.Error()))
			return
		}

		key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			window.SetContent(cannot("无法打开注册表键: " + err.Error()))
			return
		}
		defer key.Close()
		pathValue, _, err := key.GetStringValue("Path")
		if err != nil && err != registry.ErrNotExist {
			window.SetContent(cannot("无法获取 Path 值: " + err.Error()))
			return
		}
		if !strings.Contains(pathValue, `%USERPROFILE%\`+targetEnvDir) {
			pathValue += `;%USERPROFILE%\` + targetEnvDir
			err = key.SetStringValue("Path", pathValue)
			if err != nil {
				window.SetContent(cannot("无法设置新的 Path 值: " + err.Error()))
				return
			}
		}
		txt := canvas.NewText("安装成功", theme.ForegroundColor())
		txt.Alignment = fyne.TextAlignCenter
		txt.TextSize = targetTextSize / 3 * 4
		button := widget.NewButton("退出", func() { os.Exit(-1) })
		button2 := widget.NewButton("启动CMD", func() {
			cmd := exec.Command("cmd", "/C", "start", "cmd.exe")
			cmd.SysProcAttr = &syscall.SysProcAttr{
				HideWindow: false,
			}
			cmd.Start()
		})
		content := container.NewCenter(
			container.NewVBox(
				txt,
				widget.NewLabel(""),
				button,
				button2,
			),
		)
		window.SetContent(content)
	})
	button2 := widget.NewButton("退出", func() { os.Exit(-1) })
	content := container.NewCenter(
		container.NewVBox(
			txt,
			txt2,
			widget.NewLabel(""),
			txt3,
			widget.NewLabel(""),
			button,
			button2,
		),
	)
	window.SetContent(content)
	window.Resize(fyne.NewSize(targetWidth, targetHeight))
	window.SetFixedSize(true)
	if desk, ok := Env_Installer.(desktop.App); ok {
		m := fyne.NewMenu("ADB与Fastboot环境安装器", fyne.NewMenuItem("显示界面", func() { window.Show() }), fyne.NewMenuItem("隐藏界面", func() { window.Hide() }))
		desk.SetSystemTrayMenu(m)
	}
	window.CenterOnScreen()
	window.ShowAndRun()
}
