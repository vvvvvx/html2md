package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
	"github.com/dlclark/regexp2"
)

var (
	conv   = md.NewConverter("", true, nil)
	input  = flag.String("input", "", "要转换的html根目录\nHtml root dir to convert\n    Default: --input = Current dir")
	output = flag.String("output", "", "保存Markdown的目标目录\nWhere to save the MD files\n    Default: --output = Current dir")
	nameby = flag.String("nameby", "", "Markdown文件命名方式:\nHow to name MD file:\n    html = 使用html本身文件名，适用于一个目录内多个html的情况\n           Name MD by html file-name\n    dir  = 使用html父目录作为文件名，适用于一个目录只有一个html文件，且文件名为index.html的情况\n           Name MD by dir-name which html file belong to\n    Default: --nameby=html ")
)

// usage
// html2md --input '~/user/xx/' --output '~/user/yy'
func main() {
	flag.Parse()
	if *input == "" {
		*input, _ = os.Getwd()
	}
	if *output == "" {
		*output, _ = os.Getwd()
	}
	if *nameby == "" {
		*nameby = "html"
	} else if !(*nameby == "dir" || *nameby == "html") {

		fmt.Println("usage:\n  html2md --input '~/user/xx/' --output '~/user/yy' --nameby='dir'")
		flag.PrintDefaults()
		fmt.Println("\n--nameby 参数不正确。")
		return
	}
	fmt.Println("usage:\n  html2md --input '~/user/xx/' --output '~/user/yy' --nameby='dir'")
	flag.PrintDefaults()

	// Use the `GitHubFlavored` plugin from the `plugin` package.
	conv.Use(plugin.GitHubFlavored())
	p, _ := splitPath(*input, "/")
	if p == *output {
		fmt.Println("创建用于保存Markdown的文件夹与待转换的HTML文件夹同名，且位于同一父目录，请指定一个单独的文件夹存放Markdown。")
		flag.PrintDefaults()
		return
	}
	fmt.Println("Converting...")
	convertAllFile(*input, *output)
	fmt.Println("All converted")

}

// rootToConvert=要转换的当前根目录
// rootSave=要保存的根目录
func convertAllFile(rootToConvert, rootSave string) error {
	rd, err := ioutil.ReadDir(rootToConvert)
	if err != nil {
		fmt.Println("convertAllFile Error", err)
		return err
	}
	for _, fi := range rd {
		if fi.IsDir() {
			if IsSourceDir(fi.Name()) { //如果是资源文件夹，则拷贝到rootSave，只有在以文件夹命名md时才拷贝
				continue
			}
			fmt.Printf("[%s]\n", rootToConvert+"/"+fi.Name())
			_, lastDir := splitPath(rootToConvert, "/")
			convertAllFile(rootToConvert+"/"+fi.Name(), path.Join(rootSave, lastDir))
		} else {
			_, lastDir := splitPath(rootToConvert, "/") //获取待转换路径文件夹名称
			if *nameby == "html" && IsHtml(fi.Name()) {
				fname, _ := splitPath(fi.Name(), ".")
				htmlToMarkDown(path.Join(rootSave, lastDir), path.Join(rootToConvert, fi.Name()), fname) //最后一级文件夹名称作为Markdown文件名
			} else if IsHtml(fi.Name()) { //如果是html文件
				htmlToMarkDown(rootSave, path.Join(rootToConvert, fi.Name()), lastDir) //最后一级文件夹名称作为Markdown文件名
			}
		}
	}
	return err
}

// saveRoot=另存目录的根目录
// filePath=html文件的路径+文件名
// docName=html文件的名称
func htmlToMarkDown(saveRoot, filePath, docName string) error {
	//判断文件类型
	ext := path.Ext(filePath)
	if !IsHtml(ext) {
		fmt.Println(filePath, "[Not html,ignored]")
		return nil
	}
	//读取html内容
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer file.Close()

	fileinfo, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return err
	}

	filesize := fileinfo.Size()
	buffer := make([]byte, filesize)

	_, err = file.Read(buffer) //html内容保存到buffer
	if err != nil {
		fmt.Println(err)
		return err
	}

	rule := md.Rule{
		Filter: []string{"textarea"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			// You need to return a pointer to a string (md.String is just a helper function).
			// If you return nil the next function for that html element
			// will be picked. For example you could only convert an element
			// if it has a certain class name and fallback if not.
			return md.String(content)
		},
	}

	conv.AddRules(rule)
	//把buffer内的html转换为Markdown
	markdown, err := conv.ConvertString(string(buffer))
	if err != nil {
		return WrapErr("ConvertString", err)
	}
	if !strings.HasSuffix(docName, ".md") {
		docName = docName + ".md"
	}
	if err = os.MkdirAll(saveRoot, 0777); err != nil { //创建目标目录
		fmt.Println("htmlToMarkDown() Error:", err)
		return err
	}
	markdown = strings.ReplaceAll(markdown, "\\", "")

	//更改markdown图片引用路径到统一资源存放路径：src_files/
	//markdown = replPicRef(markdown)
	//markdown = replInnerPic(markdown) //恢复前一步对嵌入式图片的路径错误处理，嵌入式图片格式:  ![](data:image/png;base64,iVBORw0KGgoAA... )
	markdown, err = modifyPicRefPath(markdown)
	if err != nil {
		fmt.Println("modiPicRefPath() Error:", err)
		return err
	}
	//md文件写入到目标目录
	if err := os.WriteFile(path.Join(saveRoot, docName), []byte(markdown), 0644); err != nil {
		return WrapErr("WriteFile err", err)
	}
	fmt.Println(filePath, "[Converted]")

	//拷贝本html相关资源到目标目录
	fmt.Println("Copying related resource files.")

	sPath, fullFileName := splitPath(filePath, "/") //sPath=源文件目录，fullFileName=包括扩展名的html文件名
	fileName, _ := splitPath(fullFileName, ".")     //文件名（无扩展名）
	resPath := path.Join(sPath, fileName+"_files")
	fs, err := os.Stat(resPath) //检查是否存html在同名"xxx_files"资源目录
	if err == nil && fs.IsDir() {
		//拷贝图片资源文件到统一存放目录src_files/
		err = copyDir(resPath, path.Join(saveRoot, "src_files", fileName+"_files")) //"xxxxx_files"文件夹统一放到“resource_files”下面
		fmt.Println(path.Join(sPath, fileName+"_files", "  [Resource folder copied]"))
	}

	return err
}

/**
 * 拷贝文件夹,同时拷贝文件夹中的文件
 * @param srcPath  		需要拷贝的文件夹路径: D:/test
 * @param destPath		拷贝到的位置: D:/backup/
 */
func copyDir(srcPath string, destPath string) error {
	//检测目录正确性
	if srcInfo, err := os.Stat(srcPath); err != nil {
		fmt.Println(err.Error())
		return err
	} else {
		if !srcInfo.IsDir() {
			e := errors.New("srcPath不是一个正确的目录！")
			fmt.Println(e.Error())
			return e
		}
	}
	/**目标文件免检查，不存在可以创建
	if destInfo, err := os.Stat(destPath); err != nil {
		fmt.Println(err.Error())
		return err
	} else {
		if !destInfo.IsDir() {
			e := errors.New("destInfo不是一个正确的目录！")
			fmt.Println(e.Error())
			return e
		}
	}
	*/
	//加上拷贝时间:不用可以去掉
	//destPath = destPath + "_" + time.Now().Format("20060102150405")

	err := filepath.Walk(srcPath, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if !f.IsDir() && IsSrcFiles(f.Name()) { //不是文件夹，且是图片等资源文件，才进行复制
			path := strings.Replace(path, "\\", "/", -1)
			destNewPath := strings.Replace(path, srcPath, destPath, -1)
			fmt.Println("复制文件:" + path + " 到 " + destNewPath)
			copyFile(path, destNewPath)
		}
		return nil
	})
	if err != nil {
		fmt.Println(err.Error())
	}
	return err
}

// 生成目录并拷贝文件
func copyFile(src, dest string) (w int64, err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer srcFile.Close()
	//分割path目录
	destSplitPathDirs := strings.Split(dest, "/")

	//检测时候存在目录
	destSplitPath := ""
	for index, dir := range destSplitPathDirs {
		if index < len(destSplitPathDirs)-1 {
			destSplitPath = destSplitPath + dir + "/"
			b, _ := pathExists(destSplitPath)
			if !b {
				fmt.Println("创建目录:" + destSplitPath)
				//创建目录
				err := os.MkdirAll(destSplitPath, os.ModePerm)
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}
	dstFile, err := os.Create(dest)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer dstFile.Close()

	return io.Copy(dstFile, srcFile)
}

// 匹配图片连接:![abc](yyy_files/xxx.jpg)的特征 ![abc](
// 或         ![[abc]](yyy_files/xxx.png)的特征 ![[abc]](
// 但不匹配：   ![abc](data:image/png; base64,iVBORw0KG....) 的特征 ![abc](data:
func modifyPicRefPath(str string) (string, error) {
	//reg := regexp2.MustCompile(`(\!\[[-_\w一-龥]*\]\()(?!data:)`, 1)
	//reg := regexp2.MustCompile(`(\!\[[-_\w{han}！“”。，：；]*\]\()(?!data:)`, 1)
	reg := regexp2.MustCompile(`(\!\[{1,2}[-_.\w{han}！\”。，：；]*\]{1, }\()(?!data:)`, 1) // ![[文字]]() 或 ![文字]()，所以'['和']'可以出现1到两次
	return reg.Replace(str, `${1}src_files/`, 0, -1)
}

func IsHtml(fName string) bool {
	reg := regexp.MustCompile("(.html$)|(.htm$)|(.xhtml$)|(.xhtm$)|(.shtml$)")
	return reg.MatchString(strings.ToLower(fName))

}

// 判断是否为html的资源文件夹
func IsSourceDir(dirName string) bool {
	reg := regexp.MustCompile("_files$")
	return reg.MatchString(strings.ToLower(dirName))
}

// 判断是否为html的资源文件，只过滤图片
func IsSrcFiles(fileName string) bool {
	reg := regexp.MustCompile("(.avif$)|(.png$)|(.jpg$)|(.jpeg$)|(.jfif$)|(.pjpeg$)|(.pjp$)|(.gif$)|(.svg$)|(.svgz$)|(.bmp$)|(.pdf$)|(.eps$)|(.tiff$)|(.tif$)|(.ico$)|(.webp$)|(.xbm$)")
	return reg.MatchString(strings.ToLower(fileName))
}

// lastDir=路径最后一级的文件夹名称
// parentPath=截取lastDir后剩下的前段
func splitPath(sPath, spliter string) (parentPath, lastDir string) {

	idx := strings.LastIndex(sPath, spliter)
	//cPath := []rune(sPath)
	parentPath = sPath[0:idx]
	lastDir = sPath[idx+1:]
	return

}

// 检测文件夹路径是否存在
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func PanicErr(err error) {
	if err != nil {
		panic(err)
	}
}

func WrapErr(errMsg string, err error) error {
	if err != nil {
		return errors.New(errMsg + ", err: " + err.Error())
	}
	return nil
}
