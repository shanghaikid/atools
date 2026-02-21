# init · start

## `agix init`

创建默认配置文件 `~/.agix/config.yaml`，目录权限 `0700`，文件权限 `0600`。

```bash
agix init
```

生成的配置文件包含所有可选字段的注释说明，可直接在此基础上修改。详见[配置文件参考](/agix/config)。

## `agix start`

启动反向代理服务器，监听配置文件中指定的端口。

```bash
agix start              # 使用 config.yaml 中的 port
agix start --port 9090  # 覆盖端口
```

| 选项 | 说明 |
|------|------|
| `--port <n>` | 监听端口（覆盖配置文件） |

启动后代理默认监听 `http://localhost:8080/v1`，将该地址作为 OpenAI SDK 的 `base_url` 即可接入：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="unused",
)
```

或通过环境变量（无需改代码）：

```bash
export OPENAI_BASE_URL=http://localhost:8080/v1
```
