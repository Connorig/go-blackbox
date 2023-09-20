可以将Vue项目打包好的dist文件下的 assets/* favicon.ico index.html 放在 static包下，
在主函数加入EnableStatic()方法引入static.StaticFile。Iris会自动配置静态资源路径映射，
通过浏览器 https://localhost:端口/ 可直接访问vue发布的项目。

测试
