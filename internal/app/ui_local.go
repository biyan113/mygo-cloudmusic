package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (a *app) localPage(c *ui.Context) {
	if len(a.local) > 0 {
		a.trackRows(c, a.local, false, a.localHeader)
		return
	}
	top, bottom := a.insets(c)
	ui.Column(c).Grow(1).Padding(top+pageGap, pagePad, bottom, pagePad).Children(func() {
		a.localHeader(c)
		ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
			ui.Icon(c, iconNote).Size(42, 42).TextColor(a.pal.faint)
			ui.Text(c, "添加音乐文件或整个文件夹，无需联网即可播放").FontSize(13).TextColor(a.pal.muted)
			ui.Row(c).Gap(12).Children(func() {
				if pillButton(c.Key("add-empty"), iconPlus, "添加音乐", a.pal.accent, a.pal.onAccent).Clicked() {
					go a.chooseLocal()
				}
				if pillButton(c.Key("add-folder-empty"), iconFolder, "添加文件夹", a.pal.press, a.pal.text).Clicked() {
					go a.chooseLocalFolder()
				}
			})
		})
	})
}

func (a *app) localHeader(c *ui.Context) {
	a.heading(c, "本地音乐", fmt.Sprintf("%d 首 · MP3 / FLAC", len(a.local)))
	ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(0, 0, 20, 0).Children(func() {
		if pillButton(c.Key("add"), iconPlus, "添加音乐", a.pal.accent, a.pal.onAccent).Clicked() {
			go a.chooseLocal()
		}
		if pillButton(c.Key("add-folder"), iconFolder, "添加文件夹", a.pal.press, a.pal.text).Clicked() {
			go a.chooseLocalFolder()
		}
		if a.localErr != "" {
			ui.Text(c, a.localErr).FontSize(12).TextColor(a.pal.accent).SingleLine()
		}
	})
}
