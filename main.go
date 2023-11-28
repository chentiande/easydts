package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Configuration struct {
	Listenport string `xml:"listenport"`
	Dbip       string `xml:"dbip"`
	Dbport     string `xml:"dbport"`
	Dbname     string `xml:"dbname"`
	Dbuser     string `xml:"dbuser"`
	Dbpwd      string `xml:"dbpwd"`
}

func getconf(filename string) (Configuration, error) {
	xmlFile, err := os.Open(filename)
	var conf Configuration
	if err != nil {
		log.Println("Error opening file:", err)
		return conf, err
	}
	defer xmlFile.Close()

	if err := xml.NewDecoder(xmlFile).Decode(&conf); err != nil {
		log.Println("Error Decode file:", err)
		return conf, err
	}

	return conf, nil

}

type Report_Table struct {
	ReportID       string `json:"id" gorm:"primary_key;column:report_id;comment:报表编码;size:64;"`
	ReportName     string `json:"name" gorm:"column:report_name;comment:报表名称;size:64;"`
	CreationTime   string `json:"createtime" gorm:"column:create_time;comment:创建时间 ;size:20;"`
	DatabaseID     string `json:"dbid" gorm:"column:db_id;comment:数据源编码;size:64;"`
	Creator        string `json:"creator" gorm:";column:creator;comment:创建者;size:64;"`
	Header         string `json:"header" gorm:";column:header;comment:表头;size:1000;"`
	Status         uint   `json:"status" gorm:"column:status;comment:状态 0待审批 1可用 2停用;"`
	IsPage         uint   `json:"ispage" gorm:"column:is_page;comment:是否分页 1分页 0不分页;"`
	ExecSQL        string `json:"execsql" gorm:"column:exec_sql;comment:执行sql;size:2000;"`
	Columns        string `json:"columns" gorm:"column:columns;comment:查询字段;size:1000;"`
	QueryCondition string `json:"querycondition" gorm:"column:query_condition;comment:查询条件;size:1000;"`
	QueryTime      string `json:"querytime" gorm:"column:query_time;comment:查询时间默认值;size:1000;"`
}

func (Report_Table) TableName() string {
	return "app_report_table"
}

type Database_Config struct {
	DatabaseID string `json:"dbid" gorm:"primary_key;column:db_id;comment:数据源编码;size:64;"`
	Status     uint   `json:"status" gorm:"column:status;comment:状态 0待审批 1可用 2停用;"`
	DbType     uint   `json:"dbtype" gorm:"column:db_type;comment:数据库类型 0 pg，1 mysql;"`
	ConnString string `json:"Connstr" gorm:"column:conn_str;comment:数据库连接串;size:1000;"`
}

func (Database_Config) TableName() string {
	return "app_db_cfg"
}

type MyMux struct {
	reportinfo []Report_Table
	dbinfo     []Database_Config
	mysqldb    *gorm.DB
}

func MD5V(b string) string {
	h := md5.New()
	h.Write([]byte("cmcc" + b))
	return hex.EncodeToString(h.Sum(nil))
}

func getdata(dbtype uint, ConnString string, ExecSQL string, starttime string, endtime string) string {
	//fmt.Println(fmt.Sprintf("%v", dbtype), ConnString, ExecSQL, starttime, endtime)
	data := ""

	str := ConnString
	if dbtype == 1 {
		db, err := sql.Open("mysql", str)
		if err != nil {
			log.Println(err.Error)
			return ""
		}
		defer db.Close()

		num := 0

		ExecSQL = strings.ReplaceAll(ExecSQL, "{{starttime}}", strings.Replace(starttime, "+", " ", -1))
		ExecSQL = strings.ReplaceAll(ExecSQL, "{{endtime}}", strings.Replace(endtime, "+", " ", -1))
		fmt.Println(ExecSQL)
		rows, err := db.Query(ExecSQL)
		if err != nil {
			log.Println(err.Error())
			return ""
		}
		cols, _ := rows.Columns()
		rawResult := make([][]byte, len(cols))
		result := make([]string, len(cols))
		dest := make([]interface{}, len(cols))
		for i := range rawResult {
			dest[i] = &rawResult[i]
		}

		for rows.Next() {

			err = rows.Scan(dest...)
			num = num + 1
			data = data + "{"
			for i, raw := range rawResult {

				if raw == nil || string(raw) == "" {
					result[i] = ""
					data = data + "\"" + cols[i] + "\":" + "\"\","
				} else {

					result[i] = string(raw)
					data = data + "\"" + cols[i] + "\":" + "\"" + string(raw) + "\","

				}

			}
			data = data[:len(data)-1] + "},"

		}

	}
	if dbtype == 0 {

		db, err := sql.Open("postgres", str)
		if err != nil {
			log.Println(err.Error())
			return ""
		}
		defer db.Close()

		num := 0

		ExecSQL = strings.ReplaceAll(ExecSQL, "{{starttime}}", starttime)
		ExecSQL = strings.ReplaceAll(ExecSQL, "{{endtime}}", endtime)
		fmt.Println(ExecSQL)
		rows, err := db.Query(ExecSQL)
		fmt.Println(rows.Columns())
		if err != nil {
			log.Println(err.Error)
			return ""
		}
		cols, _ := rows.Columns()
		rawResult := make([][]byte, len(cols))
		result := make([]string, len(cols))
		dest := make([]interface{}, len(cols))
		for i := range rawResult {
			dest[i] = &rawResult[i]
		}

		for rows.Next() {

			err = rows.Scan(dest...)
			num = num + 1
			data = data + "{"
			for i, raw := range rawResult {

				if raw == nil || string(raw) == "" {
					result[i] = ""
					data = data + "\"" + cols[i] + "\":" + "\"\","
				} else {

					result[i] = string(raw)
					data = data + "\"" + cols[i] + "\":" + "\"" + string(raw) + "\","

				}

			}
			data = data[:len(data)-1] + "},"

		}

	}
	if len(data) > 1 {

		return "{\"data\":[" + data[:len(data)-1] + "]}"
	} else {
		return "{\"data\":[]}"
	}

	//

}

func gettbinfo(dbtype uint, ConnString string, qtype string, key string, key1 string) string {
	fmt.Println(fmt.Sprintf("%v", dbtype), ConnString, qtype, key)
	data := ""
	var db *sql.DB
	var err error
	str := ConnString
	if dbtype == 1 {
		db, err = sql.Open("mysql", str)
		if err != nil {
			log.Println(err.Error)
			return ""
		}
		defer db.Close()
	} else if dbtype == 0 {
		db, err = sql.Open("postgres", str)
		if err != nil {
			log.Println(err.Error())
			return ""
		}
	}

	num := 0
	ExecSQL := ""

	if dbtype == 1 {
		switch qtype {
		case "1":
			ExecSQL = "select distinct table_schema dbs from INFORMATION_SCHEMA.TABLES where table_schema not in ('information_schema','performance_schema','sys')"
		case "2":
			ExecSQL = "select distinct  TABLE_NAME cnname,  if(length(taBLE_COMMENT)>0,taBLE_COMMENT,TABLE_NAME) zhname from INFORMATION_SCHEMA.TABLES where table_schema ='" + key + "'"
		case "3":
			ExecSQL = "select distinct  COLUMN_NAME cnname,  if(length(COLUMN_COMMENT)>0,COLUMN_COMMENT,COLUMN_NAME) zhname, COLUMN_TYPE type from   INFORMATION_SCHEMA.COLUMNS where table_schema='" + key1 + "' and TABLE_NAME ='" + key + "'"

		}
	}

	if dbtype == 0 {
		switch qtype {
		case "1":
			ExecSQL = " select nspname dbs from pg_namespace  order by nspname"
		case "2":
			ExecSQL = "SELECT  relname cnname,CASE WHEN obj_description(c.oid) is null THEN relname ELSE obj_description(c.oid) END zhname FROM pg_class  c,pg_namespace n WHERE nspname = '" + key + "' AND relnamespace = n.oid and relkind='r'"
		case "3":
			ExecSQL = "select  cnname,CASE WHEN zhname is null THEN cnname ELSE zhname END,atttypid from (select  attname cnname, (select DESCription   from  pg_description d where d.objoid=a.attrelid and d.objsubid=a.attnum) zhname,atttypid  from pg_attribute a,pg_class b where b.oid=a.attrelid andnspname='" + key1 + "' relname='" + key + "' and attstattarget=-1) a"

		}
	}
	fmt.Println(ExecSQL)
	rows, err := db.Query(ExecSQL)
	if err != nil {
		log.Println(err.Error())
		return ""
	}
	cols, _ := rows.Columns()
	rawResult := make([][]byte, len(cols))
	result := make([]string, len(cols))
	dest := make([]interface{}, len(cols))
	for i := range rawResult {
		dest[i] = &rawResult[i]
	}

	for rows.Next() {

		err = rows.Scan(dest...)
		num = num + 1
		data = data + "{"
		for i, raw := range rawResult {

			if raw == nil || string(raw) == "" {
				result[i] = ""
				data = data + "\"" + cols[i] + "\":" + "\"\","
			} else {

				result[i] = string(raw)
				data = data + "\"" + cols[i] + "\":" + "\"" + string(raw) + "\","

			}

		}
		data = data[:len(data)-1] + "},"

	}

	if len(data) > 1 {

		return "[" + data[:len(data)-1] + "]"
	} else {
		return "{[]}"
	}

}

// http服务，提供API服务能力
func (p *MyMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path == "/easydts/dbadd" {

		if r.Method == "GET" {
			tmpl := `
 <!DOCTYPE html>
 <html>
<head>
    <title>数据库配置</title>
	<meta charset="UTF-8">
	<meta http-equiv="X-UA-Compatible" content="IE=edge">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body>
    <h1>数据库配置</h1>


    <form action="/easydts/dbadd" method="POST">
        <label for="dbid">数据库名称:</label>
        <input type="text" name="dbid"  style="width:400px" required><br>
        <label for="username">数据类型:</label>
		<select id="dbtype" name="dbtype">
		<option selected="selected" value="1">mysql</option>
		<option value="0">postgre</option>
		</select><br>
		<label for="constr">数据连接串:</label>
		<h4>mysql  root:***@tcp(localhost:3306)/gin?charset=utf8mb4&parseTime=True&loc=Local</h4>
		<h4>postgre  user=postgres password=*** dbname=postgres host=127.0.0.1 port=5432 sslmode=disable</h4>
        <input type="text" name="constr" value="" style="width:400px" required></input><br>
      <button type="submit">提交</button>
    </form>`
			fmt.Fprintln(w, tmpl)
			return

		} else {
			w.WriteHeader(200)
			r.ParseForm()
			dbid := r.Form.Get("dbid")
			dbtype := r.Form.Get("dbtype")

			constr := r.Form.Get("constr")

			var dbtype1 uint
			if dbtype == "0" {
				dbtype1 = 1
			}
			tmprep := &Database_Config{DatabaseID: dbid, DbType: dbtype1, ConnString: constr, Status: 1}
			p.dbinfo = append(p.dbinfo, *tmprep)
			tx := p.mysqldb.Create(tmprep)
			if tx.Error != nil {
				fmt.Fprintln(w, "添加错误"+tx.Error.Error())
				return
			} else {
				fmt.Fprintln(w, "添加成功")
				return

			}
		}

	}

	if r.URL.Path == "/easydts/add" {

		if r.Method == "GET" {
			dbname := "<select id=\"dataSource\"  style=\"width:400px\" name=\"dbid\">\n"
			dbname += "<option selected=\"selected\" value=\"\">选择数据源</option>\n"
			for _, d := range p.dbinfo {

				dbname = dbname + "<option  value=\"" + d.DatabaseID + "\">" + d.DatabaseID + "</option>\n"

			}
			dbname = dbname + "</select><br>"

			tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>报表开发</title>
	<meta charset="UTF-8">
	<meta http-equiv="X-UA-Compatible" content="IE=edge">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<script type="text/javascript" charset="utf8" src="res/jquery-1.10.2.min.js"></script>
	<script>
    $(document).ready(function() {
      // 第一个下拉框选择数据源时触发事件
      $('#dataSource').change(function() {
        var selectedDataSource = $(this).val();
        
        // 发起 AJAX 请求获取数据库名
        $.ajax({
          url: '/easydts/api/dbinfo?qtype=1', // 替换为获取数据库名的 API 地址
          method: 'GET',
          data: { dataSource: selectedDataSource },
          success: function(data) {
            // 填充第二个下拉框
            $('#database').empty();
            $.each(data, function(index, data1) {
              $('#database').append('<option value="' + data1.dbs + '">' +  data1.dbs + '</option>');
            });
			$('#database').prepend("<option selected=\"selected\" value='0'>请选择</option>");
          },
          error: function(error) {
            console.error('获取数据库名时出错：', error);
          }
        });
      });

      // 第二个下拉框选择数据库时触发事件
      $('#database').change(function() {
        var selectedDatabase = $(this).val();
        
        // 发起 AJAX 请求获取表名
        $.ajax({
          url: '/easydts/api/dbinfo?qtype=2', // 替换为获取表名的 API 地址
          method: 'GET',
          data: { dataSource:  $('#dataSource').val(),database: selectedDatabase },
          success: function(data) {
            // 填充第三个下拉框
            $('#table').empty();
            $.each(data, function(index, tableName) {
              $('#table').append('<option value="' + tableName.cnname + '">' + tableName.zhname + '</option>');
            });
			$('#table').prepend("<option selected=\"selected\" value='0'>请选择</option>");
          },
          error: function(error) {
            console.error('获取表名时出错：', error);
          }
        });
      });

      // 第三个下拉框选择表时触发事件
      $('#table').change(function() {
        var selectedTable = $(this).val();

		
		$('#reportname').val($('#table').find("option:selected").text())
		
        
        // 发起 AJAX 请求获取字段
        $.ajax({
          url: '/easydts/api/dbinfo?qtype=3', // 替换为获取字段的 API 地址
          method: 'GET',
          data: { dataSource: $('#dataSource').val(),database:  $('#database').val(),table: selectedTable },
          success: function(data) {
            // 填充第四个下拉框
            $('#fields').empty();
            $.each(data, function(index, fieldName) {
              $('#fields').append('<option value="' + fieldName.cnname + '">' + fieldName.zhname + '</option>');
            });
		
          },
          error: function(error) {
            console.error('获取字段时出错：', error);
          }
        });
      });

      // 第四个下拉框选择字段时触发事件
      $('#fields').change(function() {
        // 获取多选的字段值并用逗号连接
        var selectedFields = $('#fields').val();
	
		var selectedzhFields = [];
		$('#fields').find("option:selected").each(function () {   
			selectedzhFields.push($(this).text());    
		});
		
        var combinedFields = "select "+selectedFields.join(', ')+" from "+$('#database').val()+"."+$('#table').val();
		var zhFields ="<thead><tr><th>"+selectedzhFields.join('</th><th>')+"</th></tr></thead>";
		
        // 将组合后的字段填写到文本框中
        $('#result').val(combinedFields);
		$('#header').val(zhFields);
      });

	  $('#querycondition').change(function() {
        var selectedDatabase = $(this).val();
		if (selectedDatabase=="1"){
			selectedDatabase=""
		}
		if (selectedDatabase=="2"){
			selectedDatabase=" where $datatime='{{starttime}}'"
		}
		if (selectedDatabase=="3"){
			selectedDatabase=" where $datatime>='{{starttime}}' and $datatime<'{{endtime}}' "
		}

		$('#result').val($('#result').val()+selectedDatabase)
		});
    });
  </script>
</head>
<body>
    <h1>报表开发</h1>
    <form action="/easydts/add" method="POST">
        <label for="username">报表名称:</label>
        <input type="text" id="reportname" name="reportname"  style="width:400px" required><br>
        <label for="username">数据源:</label>
` + dbname + `
<label for="username">数据库:</label>
<select id="database"  style="width:400px"></select><br>
<label for="username">数据表:</label>
<select id="table"  style="width:400px"></select><br>
<label for="username">数据字段:</label><br>
<select id="fields" multiple="multiple"  style="height:80px;width:400px"></select><br>


        <label for="username">表头:</label><br>
       	<textarea cols="80" rows="20" id="header" name="header" required><thead><tr><th>字段1</th><th>字段2</th></tr></thead></textarea><br>
				<label for="username">执行sql:</label><br>
		<textarea cols="80" rows="20" name="execsql"  id="result" required>select * from table where starttime='{{starttime}}'</textarea><br>
        
        <input type="hidden" name="columns" style="width:400px" value="#" required>
        <label for="username">查询条件:</label>
     

		<select id="querycondition" name="querycondition"  style="width:400px">
		<option value="">请选择:</option>         
		<option value="1">无时间选择</option>         
		<option value="2">开始时间</option> 
		<option value="3">开始结束时间</option> 
		</select><br>
		<label for="username">查询时间默认值:</label>
        <input type="text" name="querytime" value="NOW|60|-120" style="width:400px" required></input><br>
  


        <button type="submit">提交</button>
    </form>
	
	
	
	
	`
			fmt.Fprintln(w, tmpl)
			return

		} else {
			w.WriteHeader(200)
			r.ParseForm()
			reportname := r.Form.Get("reportname")
			dbid := r.Form.Get("dbid")
			header := r.Form.Get("header")
			execsql := r.Form.Get("execsql")
			columns := r.Form.Get("columns")
			querycondition := r.Form.Get("querycondition")
			querytime := r.Form.Get("querytime")
			for _, d := range p.dbinfo {
				if dbid == d.DatabaseID {
					if d.DbType == 1 {
						db, err := sql.Open("mysql", d.ConnString)
						if err != nil {
							log.Println(err.Error)

						}
						defer db.Close()

						ExecSQL := strings.ReplaceAll(execsql, "{{starttime}}", "1997-01-01 00:00:00")
						ExecSQL = strings.ReplaceAll(ExecSQL, "{{endtime}}", "1997-01-01 00:00:00")
						if strings.Index(strings.ToLower(ExecSQL), " where ") > 1 {
							ExecSQL = ExecSQL + " and 1=2"
						} else {
							ExecSQL = ExecSQL + " where 1=2"
						}
						rows, err := db.Query(ExecSQL)
						if err != nil {
							log.Println(err.Error())
							fmt.Fprintln(w, "添加错误"+err.Error())
							return
						}
						cols, _ := rows.Columns()

						if !(strings.Index(strings.ToLower(header), "</thead>") > 1) {
							header = "<thead><tr><th>" + strings.Join(cols, "</th><th>") + "</th></tr></thead>"
						}
						if columns == "#" {
							columns = strings.Join(cols, ",")
						}

						//execsql = strings.ReplaceAll(execsql, "*", strings.Join(cols, ","))

					}
					if d.DbType == 0 {
						db, err := sql.Open("postgres", d.ConnString)
						if err != nil {
							log.Println(err.Error)

						}
						defer db.Close()
						execsql = strings.ReplaceAll(execsql, ";", "")
						ExecSQL := strings.ReplaceAll(execsql, "{{starttime}}", "1997-01-01 00:00:00")
						ExecSQL = strings.ReplaceAll(ExecSQL, "{{endtime}}", "1997-01-01 00:00:00")
						if strings.Index(strings.ToLower(ExecSQL), " where ") > 1 {

							ExecSQL = ExecSQL + " and 1=2"
						} else {
							ExecSQL = ExecSQL + " where 1=2"
						}
						rows, err := db.Query(ExecSQL)
						if err != nil {
							log.Println(err.Error())
							fmt.Fprintln(w, "添加错误"+err.Error())
							return
						}
						cols, _ := rows.Columns()

						if !(strings.Index(strings.ToLower(header), "<thead>") > 1) {
							header = "<thead><tr><th>" + strings.Join(cols, "</th><th>") + "</th></tr></thead>"
						}
						if columns == "#" {
							columns = strings.Join(cols, ",")
						}

						execsql = strings.ReplaceAll(execsql, "*", strings.Join(cols, ","))

					}

				}
			}
			tmprep := &Report_Table{ReportID: MD5V(reportname), ReportName: reportname, DatabaseID: dbid, Header: header, Status: 1, IsPage: 0, Columns: columns, ExecSQL: execsql, QueryCondition: querycondition, QueryTime: querytime}
			p.reportinfo = append(p.reportinfo, *tmprep)
			tx := p.mysqldb.Create(tmprep)
			if tx.Error != nil {
				fmt.Fprintln(w, "添加错误"+tx.Error.Error())
				return
			} else {
				fmt.Fprintln(w, "添加成功，请访问地址为/easydts/list?id="+MD5V(reportname))
				return

			}
		}

	}

	if r.URL.Path == "/easydts/api/mydata" {

		w.Header().Set("content-type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		r.ParseForm()
		id := r.Form.Get("id")
		starttime := r.Form.Get("starttime")
		endtime := r.Form.Get("endtime")

		if id != "" {
			for _, r2 := range p.reportinfo {
				if strings.ToLower(r2.ReportID) == id {

					for _, d := range p.dbinfo {
						if r2.DatabaseID == d.DatabaseID {

							if starttime == "" {
								if len(r2.QueryTime) > 2 && strings.ToUpper(r2.QueryTime[:3]) == "NOW" {

									allp1 := strings.Split(r2.QueryTime, "|")
									if len(allp1) > 2 {
										inteval_int, _ := strconv.Atoi(allp1[1])
										number_int, _ := strconv.Atoi(allp1[2])

										t2 := time.Now().Add(time.Minute * time.Duration(number_int))
										if starttime == "" {
											starttime = t2.Add(time.Minute * time.Duration(t2.Minute()%inteval_int*-1)).Format("2006-01-02 15:04:00")

										}
										if endtime == "" {
											endtime = t2.Add(time.Minute * time.Duration(t2.Minute()%inteval_int*-1+inteval_int)).Format("2006-01-02 15:04:00")

										}
									}

								}
							}

							data := getdata(d.DbType, d.ConnString, r2.ExecSQL, starttime, endtime)
							fmt.Fprintln(w, data)
							return

						}
					}

				}
			}
		}

		return
	}

	if r.URL.Path == "/easydts/api/dbinfo" {

		w.Header().Set("content-type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		r.ParseForm()
		qtype := r.Form.Get("qtype")
		dataSource := r.Form.Get("dataSource")
		database := r.Form.Get("database")
		table := r.Form.Get("table")

		var dbtype uint
		var ConnString string
		for _, d := range p.dbinfo {
			if dataSource == d.DatabaseID {
				dbtype = d.DbType
				ConnString = d.ConnString
			}
		}
		if qtype == "1" {
			data := gettbinfo(dbtype, ConnString, "1", "", "")
			fmt.Fprintln(w, data)
		}
		if qtype == "2" {

			data := gettbinfo(dbtype, ConnString, "2", database, "")
			fmt.Fprintln(w, data)
		}
		if qtype == "3" {

			data := gettbinfo(dbtype, ConnString, "3", table, database)
			fmt.Fprintln(w, data)
		}
		return
	}

	//创建用户服务
	if r.URL.Path == "/easydts/list" {

		r.ParseForm()
		//id := r.Form.Get("id")
		id := r.Form.Get("id")

		header := ""
		name := ""
		columns := ""
		querycondition := ""

		if id != "" {
			for _, r2 := range p.reportinfo {
				if strings.ToLower(r2.ReportID) == id {

					for _, d := range p.dbinfo {
						if r2.DatabaseID == d.DatabaseID {

							header = r2.Header
							name = r2.ReportName
							columns = r2.Columns
							querycondition = r2.QueryCondition

						}
					}

				}
			}
		}

		col := strings.Split(columns, ",")
		columns = ""
		for _, v := range col {
			columns = columns + "{ data: '" + v + "'},"
		}

		datapicker := ""
		switch querycondition {
		case "2":
			datapicker = `<label>时间选择:</label>
			<input id="datetimepicker1" name="starttime" type="text" >
			<button onclick="reloadTable()">查询</button>`
		case "3":
			datapicker = `<label>开始时间:</label>
			<input id="datetimepicker1" name="starttime" type="text" >
			<label>结束时间:</label>
			<input id="datetimepicker2" name="endtime" type="text" >
			<button onclick="reloadTable()">查询</button>`
		}

		columns = columns[:len(columns)-1]
		if name == "" {
			name = "报表"
		}
		//dt := r.Form.Get("dt")
		// //鉴权验证，如果header中没有token，拒绝服务
		// p.Users[username].Password

		fmt.Fprintln(w, `<!DOCTYPE html>
        <html lang="zh-CN">
        
        <head>
          <meta charset="UTF-8">
          <meta http-equiv="X-UA-Compatible" content="IE=edge">
          <meta name="viewport" content="width=device-width, initial-scale=1.0">
          <title>`+name+`</title>
          <link rel="stylesheet" type="text/css" href="res/datatables.min.css">
		  <link  rel="stylesheet" type="text/css" href="res/buttons.dataTables.min.css">
		  <link rel="stylesheet" type="text/css" href="res/jquery.datetimepicker.min.css">

		 
        
        
        </head>
        
        <body>

		<script type="text/javascript" charset="utf8" src="res/jquery-1.10.2.min.js"></script>

         <script type="text/javascript" charset="utf8" src="res/datatables.min.js"></script>
		 <script src="res/jquery.dataTables.min.js"></script>
		 <script src="res/dataTables.buttons.min.js"></script>
		 <script src="res/jszip.min.js"></script>
		 <script src="res/pdfmake.min.js"></script>
		 <script src="res/vfs_fonts.js"></script>
		 <script src="res/buttons.html5.min.js"></script>
		 <script type="text/javascript" charset="utf8" src="res/jquery.datetimepicker.full.min.js"></script>
		 <h3 style="text-align:center">`+name+`</h3>
		 
`+datapicker+`
		
         
		<table id="myTable" class="cell-border" style="width:100%">
		
	
		
		</br>
		
		`+header+`
		</table>
		
		
	
	
	

	<script type="text/javascript">


	$('#datetimepicker1').datetimepicker({
		format:'Y-m-d H:i:00'
	});
	$('#datetimepicker2').datetimepicker({
		format:'Y-m-d H:i:00'
	});
	$(document).ready( function () {

		
		var oTable = $('#myTable').DataTable({
			ordering: true,
			serverSide: false,
			select: true,
			paging: true,
			lengthChange: true,
			processing: true,
			
			scrollX:true,//水平滚动
			
			autoWidth:true,
 	         responsive: false,//关闭响应式效果,否则以上设置无效

			ajax: {
				method: 'POST',
				url: '/easydts/api/mydata?id=`+id+`',
				dataSrc: 'data'
			},
			'iDisplayLength':50,
			language: {
				"sProcessing": "处理中...",
				"sLengthMenu": "显示 _MENU_ 项结果",
				"sZeroRecords": "没有匹配结果",
				"sInfo": "显示第 _START_ 至 _END_ 项结果，共 _TOTAL_ 项",
				"sInfoEmpty": "显示第 0 至 0 项结果，共 0 项",
				"sInfoFiltered": "(由 _MAX_ 项结果过滤)",
				"sInfoPostFix": "",
				"sSearch": "搜索:",
				"sUrl": "",
				"sEmptyTable": "表中数据为空",
				"sLoadingRecords": "载入中...",
				"sInfoThousands": ",",
				"oPaginate": {
					"sFirst": "首页",
					"sPrevious": "上页",
					"sNext": "下页",
					"sLast": "末页"
				},
				"oAria": {
					"sSortAscending": ": 以升序排列此列",
					"sSortDescending": ": 以降序排列此列"
				}
			},		
			
			createdRow: function ( row, data, index ) {
				if ( index %2 == 0 ) {
					$('td', row).css("background-color", "#FAF9F8");
				}
			},
			columns: [ `+columns+`
			],
			dom:"<lfB<t>ip>",
			
			buttons: [
            
  
			
					]
		} );

	

	});

	function reloadTable() {
		var starttime = $("#datetimepicker1").val();
		var endtime = $("#datetimepicker2").val();
		var param = {
			"starttime": starttime,
			"endtime": endtime
		};

		var oTable=$('#myTable').DataTable();
		oTable.settings()[0].ajax.data = param;
		oTable.ajax.reload();
	}
     
    </script>

	
     
        </body>
        
        </html>`)
		return
	}

	if len(r.URL.Path) > 13 && r.URL.Path[:13] == "/easydts/res/" {
		filePath := r.URL.Path[len("/easydts/res/"):]

		file, err := os.Open("./res/" + filePath)
		defer file.Close()

		path := r.URL.Path
		request_type := path[strings.LastIndex(path, "."):]
		switch request_type {
		case ".css":
			w.Header().Set("content-type", "text/css")
		case ".js":
			w.Header().Set("content-type", "text/javascript")
		default:
			w.Header().Set("content-type", "text/html; charset=utf-8")
		}

		w.WriteHeader(200)
		if err != nil {

			fmt.Fprintf(w, "file not found")
		} else {
			bs, _ := ioutil.ReadAll(file)

			w.Write(bs)

		}
		return
	}

}

func init() {
	file := "report.log"

	logFile, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)
	if err != nil {
		panic(err)
	}
	log.SetOutput(logFile) // 将文件设置为log输出的文件
	//日志标识
	log.SetFlags(log.Ldate | log.Ltime)
	return
}

func main() {

	var init bool
	flag.BoolVar(&init, "init", false, "初始化")

	flag.Parse()
	svconf, dberr := getconf("easydts.conf")
	if dberr != nil {
		log.Println("get easydts.conf err:", dberr)
	}
	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?charset=utf8mb4&parseTime=True&loc=Local", svconf.Dbuser, svconf.Dbpwd, svconf.Dbip, svconf.Dbport, svconf.Dbname)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Println("连接数据库错误", dsn)
		panic("failed to connect database")
	}
	if init {
		db.AutoMigrate(&Report_Table{}, &Database_Config{})
		xxx := `
		<thead>
<tr>
<th>文件名</th>
<th>用户名</th>
<th>文件名</th>
<th>用户名</th>
<th>文件名</th>
<th>用户名</th>
<th>用户名</th>
<th>文件名</th>
<th>用户名</th>
<th>文件名</th>
<th>用户名</th>

</tr>
</thead>`

		db.Create(&Database_Config{DatabaseID: "mysql1", Status: 1, DbType: 1, ConnString: "root:root@tcp(localhost:3306)/gin?charset=utf8mb4&parseTime=True&loc=Local"})
		db.Create(&Database_Config{DatabaseID: "pg1", Status: 1, DbType: 0, ConnString: "user=postgres password=PoStPaWD@2302# dbname=postgres host=10.19.195.97 port=5432 sslmode=disable"})

		db.Create(&Report_Table{ReportID: "ABCD", ReportName: "测试报表", DatabaseID: "mysql1", Header: xxx, Status: 1, IsPage: 0, Columns: "file_name,user_name,a,b,c,d,e,f,g,h,i,j", ExecSQL: "SELECT file_name,user_name,file_name a,user_name b,file_name c,user_name d,file_name e,file_name f,file_name g,file_name h,file_name i,file_name j FROM app_file_send_log where data_time<='{{starttime}}'", QueryCondition: "timeandne", QueryTime: "NOW|60|-24"})
		os.Exit(0)
	}

	var reps []Report_Table
	var dbs []Database_Config

	tx := db.Find(&reps, "status=1")
	if tx.Error != nil {

		log.Println(tx.Error)
	}

	tx = db.Find(&dbs, "status=1")
	if tx.Error != nil {

		log.Println(tx.Error)
	}

	mux := &MyMux{reps, dbs, db}

	err = http.ListenAndServe(":"+fmt.Sprintf("%v", svconf.Listenport), mux)
	if err != nil {
		log.Println("easydts start err:", err)
		os.Exit(0)
	}
	log.Println(" start success ....20231119")

}
