## ChatGPT-Proxy-Executor

### 简介

本执行器是一个用于实际执行OpenAI ChatGPT API密钥操作的组件。它可以与调度器通信，根据调度器的指令执行任务，并将结果报告给调度器。

### 安装

#### 需求

- Go语言 (>=1.14)

#### 步骤

1. 克隆仓库到本地

2. 进入项目目录并编译代码

   ```bash
   cd your_project_path
   go build
   ```

3. 配置`config.json`文件，可以参考`config.json.example`

4. 运行编译后的可执行文件

   ```bash
   ./your_executable_name
   ```

### 结构

- `main.go`: 主要的执行器逻辑
- `config.json.example`: 配置文件

### 功能

- **API密钥执行**: 能够根据下发的秘钥请求openai服务
- **与调度器通信**: 接收调度器的指令并将结果报告给调度器
- **灵活配置**: 通过`config.json`文件进行灵活配置

### 配置

使用`config.json`文件进行配置，可以参考`config.json.example`文件。配置选项包括：

- `api_keys`: API密钥列表
- `executor_name`: 执行器名称
- `report_enable`: 是否上报
- `listen_addr`: 监听地址
- `report_duration`: 上报间隔
- `scheduler_center`: 调度器中心地址

