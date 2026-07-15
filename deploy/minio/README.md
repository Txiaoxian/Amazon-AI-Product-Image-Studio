# MinIO 引导初始化

`deploy/docker-compose.yml` 使用一次性服务 `minio-bootstrap`，通过
`mc mb --ignore-existing` 以幂等方式创建所需存储桶。

图片对象必须存储在 MinIO 中。MySQL 只存储元数据和对象键，
不得存储图片二进制数据。

所需的存储桶由环境变量配置：

- `MINIO_BUCKET_ORIGINALS`
- `MINIO_BUCKET_GENERATED`
- `MINIO_BUCKET_THUMBNAILS`

不要将真实的 MinIO 凭据或对象数据放入此目录。
