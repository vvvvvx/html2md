# html2md
Conver htmls or html directories to Markdown recursively
把一个或多个html文件或目录，递归转换为Markdown.

## install
```bash
go install github.com/vvvvvx/html2md@latest
```

## usage
```bash

html2md --input '~/user/xx/' --output '~/user/yy' --nameby='dir'
html2md -h

--input string
    	要转换的html根目录 / Html root dir to convert
    	    Default: --input = Current dir
--nameby string
    	Markdown文件命名方式 / How to name MD file:
    	    html = 使用html本身文件名，适用于一个目录内多个html的情况 / 
                   By html name
    	    dir  = 使用html父目录作为文件名，适用于一个目录只有一个html文件，且文件名为index.html的情况 / 
                   By dir name
    	    default: --nameby = html 
--output string
    	保存Markdown的目标目录 / Where to save the MD files
    	    Default: --output = Current dir

```

