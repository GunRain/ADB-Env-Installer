package main

import (
	"archive/zip"
	"bufio"
	_ "embed"
	"image/color"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"app.niggergo.work/sdk/nga"
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
	AppVersion     = "CNRC1.2"
	WindowWidth    = 500
	WindowtHeight  = 300
	WindowTextSize = 21

	AdbEnvDir = ".adb-env"

	AdbZipUrl = "https://googledownloads.cn/android/repository/platform-tools-latest-windows.zip"
)

//go:embed icon.png
var iconData []byte

type AppTheme struct{}

var _ fyne.Theme = (*AppTheme)(nil)

func (m *AppTheme) Font(style fyne.TextStyle) fyne.Resource {
	fontData, err := os.ReadFile(filepath.Join(os.Getenv("WINDIR"), "Fonts", "simhei.ttf"))
	if err != nil {
		return theme.DefaultTheme().Font(style)
	}
	return &fyne.StaticResource{
		StaticName:    "simhei.ttf",
		StaticContent: fontData,
	}
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
	txt := canvas.NewText(reson, theme.Color(theme.ColorNameForeground))
	txt.Alignment, txt.TextSize = fyne.TextAlignCenter, WindowTextSize/3*4

	return container.NewCenter(container.NewVBox(
		txt,
		widget.NewLabel(""),
		widget.NewButton("退出", func() { os.Exit(-1) }),
	))
}

func main() {
	AppBase := app.New()
	AppBase.SetIcon(fyne.NewStaticResource("icon.png", iconData))
	AppBase.Settings().SetTheme(&AppTheme{})
	AppWindow := AppBase.NewWindow("ADB环境安装器  " + AppVersion)

	HomeTxt1 := canvas.NewText("为当前用户安装ADB与Fastboot环境", theme.Color(theme.ColorNameForeground))
	HomeTxt1.Alignment, HomeTxt1.TextSize = fyne.TextAlignCenter, WindowTextSize
	HomeTxt2 := canvas.NewText("将会通过网络下载最新版平台工具", theme.Color(theme.ColorNameForeground))
	HomeTxt2.Alignment, HomeTxt2.TextSize = fyne.TextAlignCenter, WindowTextSize

	local, latest := "未知", "未知"

	userDir := os.Getenv("USERPROFILE")
	targetDir := filepath.Join(userDir, AdbEnvDir)
	srcProp := filepath.Join(targetDir, "source.properties")

	func() {
		scan := func(r io.Reader) string {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				if strings.TrimSpace(parts[0]) == "Pkg.Revision" {
					return strings.TrimSpace(parts[1])
				}
			}
			return "未知"
		}
		if nga.PathExist(srcProp) {
			if file, err := os.Open(srcProp); err == nil {
				local = scan(file)
				file.Close()
			}
		}
		if httpFile, err := nga.NewHttpReader(AdbZipUrl); err == nil {
			zip, err := zip.NewReader(httpFile, httpFile.Size)
			if err != nil {
				return
			}
			for _, file := range zip.File {
				if file.Name == "platform-tools/source.properties" {
					rc, err := file.Open()
					if err != nil {
						return
					}
					latest = scan(rc)
					rc.Close()
				}
			}
		}
	}()

	HomeTxt3 := canvas.NewText("当前: "+local+"   最新: "+latest, theme.Color(theme.ColorNameForeground))
	HomeTxt3.Alignment, HomeTxt3.TextSize = fyne.TextAlignCenter, WindowTextSize/3*2
	HomeTxt4 := canvas.NewText("项目: github.com/OOM-WG   作者: 安音咲汀", theme.Color(theme.ColorNameForeground))
	HomeTxt4.Alignment, HomeTxt4.TextSize = fyne.TextAlignCenter, WindowTextSize/3*2

	HomeButton1 := widget.NewButton("进行安装", func() {
		InstTxt := canvas.NewText("正在安装中，请耐心等待", theme.Color(theme.ColorNameForeground))
		InstTxt.Alignment, InstTxt.TextSize = fyne.TextAlignCenter, WindowTextSize/3*4
		AppWindow.SetContent(container.NewCenter(container.NewVBox(InstTxt)))

		if userDir == "" {
			AppWindow.SetContent(Abort("获取环境变量 USERPROFILE 失败"))
			return
		}
		if nga.PathExist(targetDir) {
			cmd := exec.Command("adb.exe", "kill-server")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			_ = cmd.Run()
			if os.RemoveAll(targetDir) != nil {
				AppWindow.SetContent(Abort("删除旧版本失败"))
				return
			}
		}
		if os.MkdirAll(targetDir, os.ModePerm) != nil {
			AppWindow.SetContent(Abort("创建安装目录失败"))
			return
		}
		zipFile := filepath.Join(targetDir, "platform-tools.zip")
		out, err := os.Create(zipFile)
		if err != nil {
			AppWindow.SetContent(Abort("创建下载文件失败"))
			return
		}
		resp, err := http.Get(AdbZipUrl)
		if err != nil {
			AppWindow.SetContent(Abort("请求下载失败"))
			return
		}
		if resp.StatusCode != http.StatusOK {
			AppWindow.SetContent(Abort("下载文件失败"))
			return
		}
		if _, err = io.Copy(out, resp.Body); err != nil {
			AppWindow.SetContent(Abort("存储文件失败"))
			return
		}
		out.Close()
		resp.Body.Close()

		rc, err := zip.OpenReader(zipFile)
		if err != nil {
			AppWindow.SetContent(Abort("打开文件失败"))
			return
		}
		var pathPrefix string
		if len(rc.File) > 0 {
			pathPrefix = strings.Split(rc.File[0].Name, "/")[0] + "/"
		}
		for _, file := range rc.File {
			fpath := filepath.Join(targetDir, strings.TrimPrefix(file.Name, pathPrefix))
			if file.FileInfo().IsDir() {
				continue
			}
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				AppWindow.SetContent(Abort("创建文件失败"))
				return
			}
			rc, err := file.Open()
			if err != nil {
				AppWindow.SetContent(Abort("打开文件失败"))
				return
			}
			if _, err = io.Copy(outFile, rc); err != nil {
				AppWindow.SetContent(Abort("解压文件失败"))
				return
			}
			outFile.Close()
			rc.Close()
		}
		rc.Close()
		os.Remove(zipFile)

		key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			AppWindow.SetContent(Abort("打开注册表键失败"))
			return
		}
		pathValue, _, err := key.GetStringValue("Path")
		if err != nil && err != registry.ErrNotExist {
			AppWindow.SetContent(Abort("获取注册表值 Path 失败"))
			return
		}
		targetPathValue := `%USERPROFILE%\` + AdbEnvDir
		if !strings.Contains(pathValue, targetPathValue) {
			if err == registry.ErrNotExist || pathValue == "" {
				pathValue = targetPathValue
			} else {
				pathValue = targetPathValue + ";" + pathValue
			}
			if key.SetStringValue("Path", pathValue) != nil {
				AppWindow.SetContent(Abort("设置注册表值 Path 失败"))
				return
			}
		}
		key.Close()

		OkTxt := canvas.NewText("安装成功", theme.Color(theme.ColorNameForeground))
		OkTxt.Alignment, OkTxt.TextSize = fyne.TextAlignCenter, WindowTextSize/3*4
		AppWindow.SetContent(container.NewCenter(container.NewVBox(
			OkTxt,
			widget.NewLabel(""),
			widget.NewButton("退出", func() { os.Exit(0) }),
			widget.NewButton("启动CMD", func() {
				cmd := exec.Command("cmd.exe", "/C", "start", "cmd.exe")
				cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
				_ = cmd.Start()
			}),
		)))
	})

	AppWindow.SetContent(container.NewCenter(container.NewVBox(
		HomeTxt1,
		HomeTxt2,
		widget.NewLabel(""),
		HomeTxt3,
		widget.NewLabel(""),
		HomeButton1,
		widget.NewButton("退出", func() { os.Exit(0) }),
		widget.NewLabel(""),
		HomeTxt4,
	)))

	AppWindow.Resize(fyne.NewSize(WindowWidth, WindowtHeight))
	AppWindow.SetFixedSize(true)
	if Desk, ok := AppBase.(desktop.App); ok {
		Desk.SetSystemTrayMenu(fyne.NewMenu("ADB环境安装器", fyne.NewMenuItem("显示界面", func() { AppWindow.Show() }), fyne.NewMenuItem("隐藏界面", func() { AppWindow.Hide() })))
	}
	AppWindow.CenterOnScreen()
	AppWindow.ShowAndRun()
}
