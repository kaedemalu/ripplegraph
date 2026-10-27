# 構文エラーのあるファイル（他のファイルの結果は返ること）
resource "aws_s3_bucket" "logs" {
  bucket = "logs"
  tags = {
