package main

import (
	"archive/zip"
	"bufio"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"
)

type Chart struct {
	Key   string
	Value int
}

type DataSource struct {
	Title       string
	ChartData   []Chart
	Description string
	ChartTitle  string
}

type WordData struct {
	Path      string
	Type      int
	Data      interface{}
	ExcelData *ExcelData
}
type ExcelData struct {
	Title string
	Data  [][]interface{}
}

// 压缩zip包
func ZipWriter(inFolder string, outZip string) error {

	// Get a Buffer to Write To
	outFile, err := os.Create(outZip)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Create a new zip archive.
	w := zip.NewWriter(outFile)

	// Add some files to the archive.
	err = addFiles(w, inFolder, "")

	if err != nil {
		return err
	}

	// Make sure to check the error on Close.
	err = w.Close()
	if err != nil {
		return err
	}
	return nil
}

// 往zip包添加文件
func addFiles(w *zip.Writer, basePath, baseInZip string) error {
	// Open the Directory
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			dat, err := ioutil.ReadFile(basePath + file.Name())
			if err != nil {
				return err
			}

			// Add some files to the archive.
			f, err := w.Create(baseInZip + file.Name())
			if err != nil {
				return err
			}
			_, err = f.Write(dat)
			if err != nil {
				return err
			}
		} else if file.IsDir() {
			// Recurse
			newBase := basePath + file.Name() + "/"
			addFiles(w, newBase, baseInZip+file.Name()+"/")
		}
	}
	return nil
}

// 将图片读出来写入新文件中
/**
 * @param pushFile 新文件路径
 * @param data 图片url或路径
 */
func replaceImage(pushFile, data string) error {
	// 读取图片
	img, err := os.Open(data)
	if err != nil {
		log.Fatal(err)
	}
	defer img.Close()
	// 创建新文件
	out, err := os.Create(pushFile)
	if err != nil {
		return err
	}
	defer out.Close()
	// 写入图片
	_, err = io.Copy(out, img)
	return err
}

func exportExcel(data *ExcelData, fileName string) error {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			return
		}
	}()
	for idx, row := range data.Data {
		cell, err := excelize.CoordinatesToCellName(1, idx+1)
		if err != nil {
			return err
		}
		f.SetSheetRow("Sheet1", cell, &row)
	}
	if err := f.SaveAs(fileName); err != nil {
		return err
	}
	return nil
}

// 将a文件替换为已有的b文件
func replaceFile(source, destination string) error {
	// Open the original file
	originalFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer originalFile.Close()
	// Create the new file
	newFile, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer newFile.Close()
	// Copy the bytes to the new file
	_, err = io.Copy(newFile, originalFile)
	if err != nil {
		return err
	}
	return nil
}

// 模板渲染：其实简单来说就是将一组文本嵌入另一组文本模版中，返回一个你期望的文本
// 函数作用：替换渲染好的模板文件
func writeTemplateToFile(source, destination string, dataSource interface{}) error {
	srcFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 读取模板内容
	content, err := ioutil.ReadAll(srcFile)
	if err != nil {
		return err
	}

	// Declare the functions used in the template
	funcMap := template.FuncMap{
		"xmlEscape": func(str string) string {
			str = strings.ReplaceAll(str, "&", "&amp;")
			str = strings.ReplaceAll(str, "<", "&lt;")
			str = strings.ReplaceAll(str, ">", "&gt;")
			str = strings.ReplaceAll(str, "\"", "&quot;")
			str = strings.ReplaceAll(str, "'", "&apos;")

			return str
		},
		"add": func(a, b int) int {
			return a + b
		},
	}

	// 解析数据到模板中
	tmpl, err := template.New(source).Funcs(funcMap).Parse((string(content)))
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(destination, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer destFile.Close()
	err = tmpl.Execute(destFile, dataSource)
	if err != nil {
		return err
	}

	// 替换结果模板文件
	writer := bufio.NewWriter(destFile)
	if err != nil {
		return err
	}
	writer.Flush()
	return nil
}

func main() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {

		data := &DataSource{Title: "golang模板渲染下载word", Description: "这里的图表不是图片哦，可以进行交互！", ChartData: []Chart{{"小明", 100}, {"小花", 88}, {"小红", 66}}, ChartTitle: "考试得分"}
		handler := []WordData{
			{"/word/document.xml", 1, data, nil},
			{"/word/charts/chart1.xml", 1, data, nil},
			{"/word/media/image1.jpg", 3, "R-C.jpg", nil},
		}
		//组装excel数据
		chartOne := &ExcelData{Title: "考试得分", Data: [][]interface{}{
			{"姓名", "得分"},
		}}
		for _, v := range data.ChartData {
			chartOne.Data = append(chartOne.Data, []interface{}{v.Key, v.Value})
		}
		handler = append(handler, WordData{"/word/embeddings/Microsoft_Excel____.xlsx", 2, nil, chartOne})
		// 模板文件夹的路径（不能动，除非需求修改）
		srcBase := "./charts/zipFiles"
		// 替换文件夹的路径
		destBase := "./charts/zipFiles-backup"

		// handle xmlFile
		for _, v := range handler {
			switch v.Type {
			case 1:
				err := writeTemplateToFile(srcBase+v.Path, destBase+v.Path, v.Data)
				if err != nil {
					panic(err)
				}
			case 2:
				//生成相应的excel文件
				err := exportExcel(v.ExcelData, srcBase+v.Path)
				if err != nil {
					panic(err)
				}
				err = replaceFile(srcBase+v.Path, destBase+v.Path)
				if err != nil {
					panic(err)
				}
			case 3:
				// 替换图片
				err := replaceImage(destBase+v.Path, v.Data.(string))
				if err != nil {
					panic(err)
				}
			default:
				panic("暂不支持该类型的文件替换")
			}
		}

		// 压缩docx为zip
		inFolder := "./charts/zipFiles-backup/"
		outZip := "./doc.zip"
		err := ZipWriter(inFolder, outZip)
		if err != nil {
			panic(err)
		}

		// 返回响应，重命名.zip为.docx就行了
		c.Header("content-disposition", `attachment; filename=`+"document.docx")
		c.File(outZip)
	})
	r.Run()
}
