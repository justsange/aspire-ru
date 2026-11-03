package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Перевод встроенных текстур материалов (имена файлов = названия в списке).
var texDirs = [][2]string{
	{"1_Wood", "1_Дерево"}, {"2_Metals", "2_Металлы"}, {"3_Stone", "3_Камень"}, {"4_Misc", "4_Разное"},
}

var texFiles = map[string]string{
	"Beech Dark": "Бук тёмный", "Beech Light": "Бук светлый", "Birch": "Берёза", "Calvados": "Кальвадос",
	"Canadian Maple": "Клён канадский", "Cherry Dark": "Вишня тёмная", "Cherry": "Вишня", "Dark Wood": "Тёмное дерево",
	"Green Wood": "Зелёное дерево", "Mahogany Light": "Красное дерево светлое", "Mahogany": "Красное дерево",
	"Maple": "Клён", "MDF": "МДФ", "Medium Wood": "Дерево среднего тона", "Oak Dark": "Дуб тёмный",
	"Oak Light": "Дуб светлый", "Oak Limed": "Дуб белёный", "Oak Medium": "Дуб средний", "Oak Soft": "Дуб мягкий",
	"Oak": "Дуб", "Padauk": "Падук", "Pine Old": "Сосна старая", "Pine": "Сосна", "Walnut Light": "Орех светлый",
	"Walnut": "Орех", "Yew": "Тис",
	"Brass Dark": "Латунь тёмная", "Brass Light": "Латунь светлая", "Brass Reflective": "Латунь зеркальная",
	"Brass": "Латунь", "Bronze": "Бронза", "Gold Satin": "Золото матовое", "Silver Satin": "Серебро матовое",
	"Steel Bright": "Сталь блестящая", "Steel": "Сталь",
	"Granite Black": "Гранит чёрный", "Granite Tan": "Гранит бежевый", "Granite": "Гранит", "Limestone": "Известняк",
	"Marble 1": "Мрамор 1", "Marble 2": "Мрамор 2", "Sandstone Dark": "Песчаник тёмный", "Sandstone": "Песчаник",
	"Slate - Black": "Сланец чёрный", "Slate - Blue": "Сланец синий", "Slate - Purple": "Сланец фиолетовый",
	"Blue Plastic": "Пластик синий", "Green Plastic": "Пластик зелёный", "Pale Blue Plastic": "Пластик голубой",
	"Red Plastic": "Пластик красный", "Yellow Plastic": "Пластик жёлтый",
}

func textureRoots(aspireDir string) []string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	ver := aspireVer(aspireDir)
	var out []string
	if ver != "" {
		p := filepath.Join(pd, "Vectric", "Aspire", "V"+ver, "BitmapTextures")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// to=true: EN->RU, to=false: RU->EN
func renameTextures(aspireDir string, toRu bool) {
	for _, root := range textureRoots(aspireDir) {
		n := 0
		for _, d := range texDirs {
			src, dst := d[0], d[1]
			if !toRu {
				src, dst = d[1], d[0]
			}
			sp, dp := filepath.Join(root, src), filepath.Join(root, dst)
			if _, err := os.Stat(sp); err == nil {
				if _, err := os.Stat(dp); os.IsNotExist(err) {
					if os.Rename(sp, dp) == nil {
						n++
					}
				}
			}
			dir := dp
			if _, err := os.Stat(dir); err != nil {
				dir = sp
			}
			ents, _ := os.ReadDir(dir)
			for _, e := range ents {
				if e.IsDir() {
					continue
				}
				ext := filepath.Ext(e.Name())
				base := strings.TrimSuffix(e.Name(), ext)
				var nb string
				if toRu {
					nb = texFiles[base]
				} else {
					for en, ru := range texFiles {
						if ru == base {
							nb = en
						}
					}
				}
				if nb == "" {
					continue
				}
				from, to := filepath.Join(dir, e.Name()), filepath.Join(dir, nb+ext)
				if _, err := os.Stat(to); err == nil {
					continue
				}
				os.Chmod(from, 0666)
				if os.Rename(from, to) == nil {
					n++
				}
			}
		}
		if toRu {
			logf("Текстуры материалов переведены (%d): %s\n", n, root)
		} else {
			logf("Текстурам материалов возвращены английские имена (%d)\n", n)
		}
	}
}
