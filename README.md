## Modg 微服务单仓依赖解析工具

### 简介

Modg 是一个简易的用于解析微服务依赖关系的工具。在微服务单仓项目中，难以发现当前提交影响的范围，使用Modg可以快速查看当前提交可能影响的服务列表，也可以集成到CICD中用于自动构建受影响的服务。

### 安装
```shell
# 安装modg
go install github.com/comeonjy/modg@latest
```

### 使用场景

> 查看某次提交可能影响的服务列表
```shell
git show --name-only [commitid] | modg
```

> 查看当前修改可能影响的服务列表
```shell
git status -s | awk '{print $2}' | modg
```

### TODO
- [ ] 利用dotLang和graphviz绘制依赖关系图
- [ ] 优化输出
