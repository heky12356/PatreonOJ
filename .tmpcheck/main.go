package main
import (
  "fmt"
  "patreonoj/internal/config"
  "patreonoj/internal/models"
)
func main(){
  if err:=config.InitConfig("config.yaml"); err!=nil { panic(err) }
  if err:=models.InitDB(); err!=nil { panic(err) }
  var s models.Submission
  if err:=models.DB.Where("id = ?", "23de51d7-a6c0-4b2d-9e4a-0eecef0a4e82").First(&s).Error; err!=nil { panic(err) }
  fmt.Println("status=", s.Status)
  fmt.Println("error_code=", s.ErrorCode)
  fmt.Println("error_msg=", s.ErrorMsg)
  fmt.Println("question_pk=", s.QuestionID)
}
