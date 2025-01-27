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
	ver            = "CNRel1.1 (2025新年特供)"
	targetWidth    = 500
	targetHeight   = 300
	targetTextSize = 21

	targetEnvDir = ".adb-env"
)

//go:embed font.ttf
var fontData []byte

//go:embed icon.png
var iconData []byte

type AppTheme struct{}

var _ fyne.Theme = (*AppTheme)(nil)

func (m *AppTheme) Font(style fyne.TextStyle) fyne.Resource {
	return &fyne.StaticResource{StaticName: "font.ttf", StaticContent: fontData}
}

func (m *AppTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (m *AppTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *AppTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func Abort(reson string) *fyne.Container {
	txt := canvas.NewText("安装失败("+reson+")", theme.Color(theme.ColorNameForeground))
	txt.Alignment, txt.TextSize = fyne.TextAlignCenter, targetTextSize/3*4

	return container.NewCenter(container.NewVBox(
		txt,
		widget.NewLabel(""),
		widget.NewButton("退出", func() { os.Exit(-1) }),
	))
}

func main() {
	var err error

	AppBase := app.New()
	AppBase.SetIcon(fyne.NewStaticResource("icon.png", iconData))
	AppBase.Settings().SetTheme(&AppTheme{})
	AppWindow := AppBase.NewWindow("ADB环境安装器  " + ver)

	HomeTxt1 := canvas.NewText("为当前用户安装ADB与Fastboot环境", theme.Color(theme.ColorNameForeground))
	HomeTxt1.Alignment, HomeTxt1.TextSize = fyne.TextAlignCenter, targetTextSize
	HomeTxt2 := canvas.NewText("将会通过网络下载最新版平台工具", theme.Color(theme.ColorNameForeground))
	HomeTxt2.Alignment, HomeTxt2.TextSize = fyne.TextAlignCenter, targetTextSize
	HomeTxt3 := canvas.NewText("官网: www.mod.latestfile.zip   作者: 安音咲汀", theme.Color(theme.ColorNameForeground))
	HomeTxt3.Alignment, HomeTxt3.TextSize = fyne.TextAlignCenter, targetTextSize/3*2
	HomeTxt4 := canvas.NewText("感谢您的使用", theme.Color(theme.ColorNameForeground))
	HomeTxt4.Alignment, HomeTxt4.TextSize = fyne.TextAlignCenter, targetTextSize/3*2

	HomeButton1 := widget.NewButton("进行安装", func() {
		InstTxt := canvas.NewText("正在安装中，请耐心等待", theme.Color(theme.ColorNameForeground))
		InstTxt.Alignment, InstTxt.TextSize = fyne.TextAlignCenter, targetTextSize/3*4
		AppWindow.SetContent(container.NewCenter(container.NewVBox(InstTxt)))

		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			AppWindow.SetContent(Abort("获取变量 USERPROFILE 值失败(值为空)"))
			return
		}
		targetDir := filepath.Join(userProfile, targetEnvDir)

		_, err = os.Stat(targetDir)
		if err == nil {
			exec.Command("adb.exe", "kill-server").Run()
			err = os.RemoveAll(targetDir)
			if err != nil {
				AppWindow.SetContent(Abort("删除文件夹失败 " + targetDir + " : " + err.Error()))
				return
			}
		}
		err = os.MkdirAll(targetDir, os.FileMode(0755))
		if err != nil {
			AppWindow.SetContent(Abort("创建文件夹失败 " + targetDir + " : " + err.Error()))
			return
		}

		zipFile := filepath.Join(targetDir, "platform-tools.zip")
		out, err := os.Create(zipFile)
		if err != nil {
			AppWindow.SetContent(Abort("创建文件失败 " + zipFile + " : " + err.Error()))
			return
		}
		resp, err := http.Get("https://googledownloads.cn/android/repository/platform-tools-latest-windows.zip")
		if err != nil {
			AppWindow.SetContent(Abort("下载文件失败: " + err.Error()))
			return
		}
		if resp.StatusCode != http.StatusOK {
			AppWindow.SetContent(Abort("下载文件失败(" + strconv.Itoa(resp.StatusCode) + ")"))
			return
		}
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			AppWindow.SetContent(Abort("储存文件失败: " + err.Error()))
			return
		}
		if out.Close() != nil {
			AppWindow.SetContent(Abort("关闭文件失败: " + err.Error()))
			return
		}
		if resp.Body.Close() != nil {
			AppWindow.SetContent(Abort("关闭请求失败: " + err.Error()))
			return
		}

		r, err := zip.OpenReader(zipFile)
		if err != nil {
			AppWindow.SetContent(Abort("打开文件失败: " + err.Error()))
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
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				AppWindow.SetContent(Abort("创建文件夹失败 " + filepath.Dir(fpath) + " : " + err.Error()))
				return
			}
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				AppWindow.SetContent(Abort("创建文件失败 " + fpath + " : " + err.Error()))
				return
			}
			rc, err := f.Open()
			if err != nil {
				AppWindow.SetContent(Abort("打开文件失败: " + err.Error()))
				return
			}
			_, err = io.Copy(outFile, rc)
			if err != nil {
				AppWindow.SetContent(Abort("解压文件失败: " + err.Error()))
				return
			}
			err = outFile.Close()
			if err != nil {
				AppWindow.SetContent(Abort("关闭文件失败: " + err.Error()))
				return
			}
			err = rc.Close()
			if err != nil {
				AppWindow.SetContent(Abort("关闭文件失败: " + err.Error()))
				return
			}
		}
		err = r.Close()
		if err != nil {
			AppWindow.SetContent(Abort("关闭文件失败: " + err.Error()))
			return
		}
		if err = os.Remove(zipFile); err != nil {
			AppWindow.SetContent(Abort("删除文件失败 " + zipFile + " : " + err.Error()))
			return
		}

		key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			AppWindow.SetContent(Abort("打开注册表键失败: " + err.Error()))
			return
		}
		pathValue, _, err := key.GetStringValue("Path")
		if err != nil && err != registry.ErrNotExist {
			AppWindow.SetContent(Abort("获取注册表值 Path 失败: " + err.Error()))
			return
		}
		targetPathValue := `%USERPROFILE%\` + targetEnvDir
		if !strings.Contains(pathValue, targetPathValue) {
			if err == registry.ErrNotExist || pathValue == "" {
				pathValue = targetPathValue
			} else {
				pathValue = targetPathValue + ";" + pathValue
			}
			err = key.SetStringValue("Path", pathValue)
			if err != nil {
				AppWindow.SetContent(Abort("设置注册表值 Path 失败: " + err.Error()))
				return
			}
		}
		err = key.Close()
		if err != nil {
			AppWindow.SetContent(Abort("关闭注册表键失败: " + err.Error()))
			return
		}

		OkTxt := canvas.NewText("安装成功", theme.Color(theme.ColorNameForeground))
		OkTxt.Alignment, OkTxt.TextSize = fyne.TextAlignCenter, targetTextSize/3*4
		AppWindow.SetContent(container.NewCenter(container.NewVBox(
			OkTxt,
			widget.NewLabel(""),
			widget.NewButton("退出", func() { os.Exit(0) }),
			widget.NewButton("启动CMD", func() {
				cmd := exec.Command("cmd", "/C", "start", "cmd.exe")
				cmd.SysProcAttr = &syscall.SysProcAttr{
					HideWindow: false,
				}
				cmd.Start()
			}),
		)))
	})

	AppWindow.SetContent(container.NewCenter(container.NewVBox(
		HomeTxt1,
		HomeTxt2,
		widget.NewLabel(""),
		HomeTxt3,
		HomeTxt4,
		widget.NewLabel(""),
		HomeButton1,
		widget.NewButton("退出", func() { os.Exit(0) }),
	)))

	AppWindow.Resize(fyne.NewSize(targetWidth, targetHeight))
	AppWindow.SetFixedSize(true)
	if Desk, ok := AppBase.(desktop.App); ok {
		Desk.SetSystemTrayMenu(fyne.NewMenu("ADB环境安装器", fyne.NewMenuItem("显示界面", func() { AppWindow.Show() }), fyne.NewMenuItem("隐藏界面", func() { AppWindow.Hide() })))
	}
	AppWindow.CenterOnScreen()
	AppWindow.ShowAndRun()
}
