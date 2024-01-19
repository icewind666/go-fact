package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/guptarohit/asciigraph"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/manifoldco/promptui"
	_ "github.com/mattn/go-sqlite3"
)

// Сотрудник
type Employee struct {
	ID       int
	FullName string
	Position string
}

// Факт
type Fact struct {
	ID         int
	Text       string
	DateAdded  time.Time
	FactRating string // Нейтральная, Положительная, Отрицательная
	EmployeeID int
}

func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", "system.db")
	if err != nil {
		panic(err)
	}

	// Создание таблиц, если они еще не существуют
	createEmployeeTableSQL := `CREATE TABLE IF NOT EXISTS employee (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,		
		"fullname" TEXT,
		"position" TEXT
	);`
	createFactTableSQL := `CREATE TABLE IF NOT EXISTS fact (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,
		"text" TEXT,
		"dateadded" DATETIME,
		"factrating" TEXT,
		"employeeid" INTEGER,
		FOREIGN KEY (employeeid) REFERENCES employee (id)
	);`

	_, err = db.Exec(createEmployeeTableSQL)
	if err != nil {
		panic(err)
	}
	_, err = db.Exec(createFactTableSQL)
	if err != nil {
		panic(err)
	}

	return db
}

func addEmployee(db *sql.DB, fullName, position string) {
	statement, _ := db.Prepare("INSERT INTO employee (fullname, position) VALUES (?, ?)")
	_, err := statement.Exec(fullName, position)
	if err != nil {
		panic(err)
	}
	fmt.Println("Добавлен сотрудник:", fullName)
}

func addFact(db *sql.DB) {
	employees := getEmployees(db)
	if len(employees) == 0 {
		fmt.Println("Нет доступных сотрудников для добавления факта.")
		return
	}

	// Выбор сотрудника
	var employeeNames []string
	employeeMap := make(map[string]Employee)
	for _, employee := range employees {
		name := fmt.Sprintf("%s (%s)", employee.FullName, employee.Position)
		employeeNames = append(employeeNames, name)
		employeeMap[name] = employee
	}

	employeePrompt := promptui.Select{
		Label: "Выберите сотрудника",
		Items: employeeNames,
	}

	_, result, err := employeePrompt.Run()
	if err != nil {
		fmt.Printf("Выбор отменен %v\n", err)
		return
	}

	selectedEmployee := employeeMap[result]

	// Запрос текста факта
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Введите текст факта: ")
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text) // Удаляем символы новой строки и лишние пробелы

	// Выбор оценки факта
	factRatings := []string{"нейтральная", "положительная", "отрицательная"}
	ratingPrompt := promptui.Select{
		Label: "Выберите оценку факта",
		Items: factRatings,
	}

	_, factRating, err := ratingPrompt.Run()
	if err != nil {
		fmt.Printf("Выбор отменен %v\n", err)
		return
	}

	// Добавление факта в БД
	statement, _ := db.Prepare("INSERT INTO fact (text, dateadded, factrating, employeeid) VALUES (?, ?, ?, ?)")
	_, err = statement.Exec(text, time.Now(), factRating, selectedEmployee.ID)
	if err != nil {
		panic(err)
	}
	fmt.Println("Добавлен факт:", text)
}

func getEmployees(db *sql.DB) []Employee {
	var employees []Employee

	rows, err := db.Query("SELECT * FROM employee")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var e Employee
		err = rows.Scan(&e.ID, &e.FullName, &e.Position)
		if err != nil {
			panic(err)
		}
		employees = append(employees, e)
	}

	return employees
}

func getFacts(db *sql.DB) {
	rows, err := db.Query("SELECT * FROM fact JOIN employee ON fact.employeeid = employee.id")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	// Создание таблицы
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Текст", "Дата Добавления", "Оценка", "Сотрудник"})

	for rows.Next() {
		var f Fact
		var e Employee
		err = rows.Scan(&f.ID, &f.Text, &f.DateAdded, &f.FactRating, &f.EmployeeID, &e.ID, &e.FullName, &e.Position)
		if err != nil {
			panic(err)
		}
		t.AppendRow([]interface{}{f.ID, f.Text, f.DateAdded.Format("2006-01-02"), f.FactRating, e.FullName})
	}

	// Отображение таблицы
	t.Render()
}

func deleteAllFacts(db *sql.DB) {
	prompt := promptui.Select{
		Label: "Точно удалить все факты?",
		Items: []string{"Нет", "Да"},
	}

	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Выбор отменен %v\n", err)
		return
	}

	if result == "Да" {
		_, err := db.Exec("DELETE FROM fact")
		if err != nil {
			panic(err)
		}
		fmt.Println("Все факты были удалены.")
	}
}

func deleteAllEmployees(db *sql.DB) {
	prompt := promptui.Select{
		Label: "Точно удалить всех сотрудников и связанные факты?",
		Items: []string{"Нет", "Да"},
	}

	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Выбор отменен %v\n", err)
		return
	}

	if result == "Да" {
		_, err = db.Exec("DELETE FROM employee")
		if err != nil {
			panic(err)
		}
		_, err = db.Exec("DELETE FROM fact")
		if err != nil {
			panic(err)
		}
		fmt.Println("Все сотрудники и связанные с ними факты были удалены.")
	}
}

func calculateEmployeeRatings(db *sql.DB) map[string]int {
	employeeRatings := make(map[string]int)

	query := `
    SELECT e.fullname, f.factrating
    FROM fact f
    JOIN employee e ON f.employeeid = e.id
    `

	rows, err := db.Query(query)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var fullName, factRating string

		err = rows.Scan(&fullName, &factRating)
		if err != nil {
			panic(err)
		}

		var ratingValue int
		switch factRating {
		case "положительная":
			ratingValue = 1
		case "отрицательная":
			ratingValue = -1
		default:
			ratingValue = 0
		}

		employeeRatings[fullName] += ratingValue
	}

	return employeeRatings
}

// Функция для добавления сотрудника с использованием promptui
func addEmployeePrompt(db *sql.DB) {
	fmt.Print("Введите ФИО сотрудника: ")
	var fullName string
	fmt.Scan(&fullName)
	fmt.Print("Введите должность сотрудника: ")
	var position string
	fmt.Scan(&position)
	addEmployee(db, fullName, position)
}

func showEmployeeRatings(db *sql.DB) {
	ratings := calculateEmployeeRatings(db)
	for fullName, rating := range ratings {
		fmt.Printf("Сотрудник %s: Рейтинг = %d\n", fullName, rating)
	}
}

func showEmployees(db *sql.DB) {
	employees := getEmployees(db)

	for id, empl := range employees {
		fmt.Printf("%d. %s\n", id, empl.FullName)
	}
}

func showRecentFactsByEmployee(db *sql.DB) {
	employees := getEmployees(db)
	if len(employees) == 0 {
		fmt.Println("Нет доступных сотрудников.")
		return
	}

	var employeeNames []string
	employeeMap := make(map[string]int) // Сопоставление имени с ID сотрудника
	for _, employee := range employees {
		name := employee.FullName
		employeeNames = append(employeeNames, name)
		employeeMap[name] = employee.ID
	}

	prompt := promptui.Select{
		Label: "Выберите сотрудника",
		Items: employeeNames,
	}

	_, selectedName, err := prompt.Run()
	if err != nil {
		fmt.Printf("Выбор отменен %v\n", err)
		return
	}

	selectedEmployeeID := employeeMap[selectedName]
	showFactsForEmployee(db, selectedEmployeeID)
}

func showFactsForEmployee(db *sql.DB, employeeID int) {
	query := `
    SELECT text, dateadded, factrating
    FROM fact
    WHERE employeeid = ? AND dateadded > DATE('now', '-1 month')
    `

	rows, err := db.Query(query, employeeID)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Текст", "Дата Добавления", "Оценка"})

	for rows.Next() {
		var text, factRating string
		var dateAdded time.Time

		err = rows.Scan(&text, &dateAdded, &factRating)
		if err != nil {
			panic(err)
		}

		t.AppendRow([]interface{}{text, dateAdded.Format("2006-01-02"), factRating})
	}

	t.Render()
}

func getEmployeeRatingHistory(db *sql.DB, employeeID int) []float64 {
	var history []float64

	// SQL запрос для получения истории рейтингов
	query := `
    SELECT dateadded, factrating
    FROM fact
    WHERE employeeid = ?
    ORDER BY dateadded
    `

	rows, err := db.Query(query, employeeID)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var totalRating float64 // Общий рейтинг на данный момент

	for rows.Next() {
		var dateAdded time.Time
		var factRatingStr string

		err = rows.Scan(&dateAdded, &factRatingStr)
		if err != nil {
			panic(err)
		}

		var factRating float64
		switch factRatingStr {
		case "положительная":
			factRating = 1
		case "отрицательная":
			factRating = -1
		default:
			factRating = 0
		}

		totalRating += factRating
		history = append(history, totalRating)
	}

	return history
}

func showGraphs(db *sql.DB) {
	employees := getEmployees(db)
	if len(employees) == 0 {
		fmt.Println("Нет доступных сотрудников.")
		return
	}

	var employeeNames []string
	employeeMap := make(map[string]int) // Сопоставление имени с ID сотрудника
	for _, employee := range employees {
		name := employee.FullName
		employeeNames = append(employeeNames, name)
		employeeMap[name] = employee.ID
	}

	prompt := promptui.Select{
		Label: "Выберите сотрудника",
		Items: employeeNames,
	}

	_, selectedName, err := prompt.Run()
	if err != nil {
		fmt.Printf("Выбор отменен %v\n", err)
		return
	}

	selectedEmployeeID := employeeMap[selectedName]
	showEmployeeGraph(db, selectedEmployeeID)
}

func showEmployeeGraph(db *sql.DB, employeeID int) {
	// Здесь вам нужно получить данные о рейтинге выбранного сотрудника
	// Например, вы можете собрать историю изменения рейтинга сотрудника
	// и использовать эти данные для создания графика.
	// Предполагается, что у вас есть функция для получения этих данных.
	ratingsData := getEmployeeRatingHistory(db, employeeID)

	if len(ratingsData) == 0 {
		fmt.Println("Нет данных для отображения графика для данного сотрудника.")
		return
	}

	graph := asciigraph.Plot(ratingsData, asciigraph.Height(10))
	fmt.Println(graph)
}

func exportFactsToCSV(db *sql.DB) {
	rows, err := db.Query("SELECT * FROM fact JOIN employee ON fact.employeeid = employee.id")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	// Создание файла CSV
	file, err := os.Create("facts.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Запись заголовка
	writer.Write([]string{"ID", "Текст Факта", "Дата Добавления", "Оценка Факта", "Сотрудник"})

	for rows.Next() {
		var id int
		var text, factRating, employeeName string
		var dateAdded time.Time

		err := rows.Scan(&id, &text, &dateAdded, &factRating, &employeeName)
		if err != nil {
			panic(err)
		}

		writer.Write([]string{strconv.Itoa(id), text, dateAdded.Format("2006-01-02"), factRating, employeeName})
	}

	fmt.Println("Факты были экспортированы в файл 'facts.csv'")
}

// removeEmployeeFact удаляет факт
func removeEmployeeFact(db *sql.DB) {
	employees := getEmployees(db)
	if len(employees) == 0 {
		fmt.Println("Нет доступных сотрудников.")
		return
	}

	var employeeNames []string
	employeeMap := make(map[string]int) // Сопоставление имени с ID сотрудника
	for _, employee := range employees {
		name := employee.FullName
		employeeNames = append(employeeNames, name)
		employeeMap[name] = employee.ID
	}

	prompt := promptui.Select{
		Label: "Выберите сотрудника",
		Items: employeeNames,
	}

	_, selectedName, err := prompt.Run()
	if err != nil {
		fmt.Printf("Выбор отменен %v\n", err)
		return
	}

	selectedEmployeeID := employeeMap[selectedName]
	employeeLastTenFacts := getTenLastFactsForEmployee(db, selectedEmployeeID)

	var employeeFacts []string
	employeeFactMap := make(map[string]int)
	for _, employeeFact := range employeeLastTenFacts {
		name := employeeFact.Text
		employeeFacts = append(employeeFacts, name)
		employeeFactMap[name] = employeeFact.ID
	}

	prompForFact := promptui.Select{
		Label: "Выберите факт для удаления",
		Items: employeeFacts,
	}
	_, selectedFact, err := prompForFact.Run()
	if err != nil {
		fmt.Printf("Выбор отменен %v\n", err)
		return
	}

	selectedFactId := employeeFactMap[selectedFact]

	fmt.Printf("Удаляем факт %v\n", selectedFactId)
	deleteFact(db, selectedFactId)

}

func deleteFact(db *sql.DB, factId int) {
	_, err := db.Exec("DELETE FROM fact WHERE id=?", factId)
	if err != nil {
		panic(err)
	}
	fmt.Println("Факты был удален.")
}

func getTenLastFactsForEmployee(db *sql.DB, employeeId int) []Fact {
	query := `
    SELECT *
    FROM fact JOIN employee ON fact.employeeid = employee.id
    WHERE employeeid = ? order by dateadded desc limit 10
    `

	rows, err := db.Query(query, employeeId)

	if err != nil {
		panic(err)
	}

	defer rows.Close()
	var facts []Fact

	for rows.Next() {
		var f Fact
		var e Employee
		err = rows.Scan(&f.ID, &f.Text, &f.DateAdded, &f.FactRating, &f.EmployeeID, &e.ID, &e.FullName, &e.Position)
		if err != nil {
			panic(err)
		}
		facts = append(facts, f)
	}

	return facts

}

func showTopEmployees(db *sql.DB, ratingType string) {
	var order string
	if ratingType == "положительная" {
		order = "DESC" // Для положительного рейтинга выбираем наибольшие значения
	} else {
		order = "ASC" // Для отрицательного рейтинга выбираем наименьшие значения
	}

	query := `
    SELECT e.fullname, SUM(CASE f.factrating 
        WHEN 'положительная' THEN 1 
        WHEN 'отрицательная' THEN -1 
        ELSE 0 END) AS total_rating
    FROM employee e
    LEFT JOIN fact f ON e.id = f.employeeid
    GROUP BY e.id
    ORDER BY total_rating ` + order + `
    LIMIT 3
    `

	rows, err := db.Query(query)
	if err != nil {
		fmt.Println("Ошибка при выполнении запроса:", err)
		return
	}
	defer rows.Close()

	fmt.Printf("Топ 3 сотрудников по %s рейтингу:\n", ratingType)
	for rows.Next() {
		var fullname string
		var totalRating int
		if err := rows.Scan(&fullname, &totalRating); err != nil {
			fmt.Println("Ошибка при чтении результатов:", err)
			return
		}
		fmt.Printf("%s: %d\n", fullname, totalRating)
	}
}

func main() {
	db := initDB()
	defer db.Close()

	for {
		actions := []string{"Добавить сотрудника",
			"Добавить факт",
			"Просмотр фактов",
			"Показать рейтинги сотрудников",
			"Графики",
			"CSV",
			"Показать факты по сотруднику за последний месяц",
			"Удалить факт сотрудника",
			"Удалить все факты",
			"Удалить всех сотрудников",
			"Топ 3 сотрудника по положительному рейтингу",
			"Топ 3 сотрудника с отрицательным рейтингом",
			"Выход"}

		prompt := promptui.Select{
			Label: "Выберите действие",
			Items: actions,
		}

		_, result, err := prompt.Run()
		if err != nil {
			fmt.Printf("Выбор отменен %v\n", err)
			return
		}

		switch result {
		case "Удалить факт сотрудника":
			removeEmployeeFact(db)

		case "CSV":
			exportFactsToCSV(db)

		case "Графики":
			showGraphs(db)

		case "Показать факты по сотруднику за последний месяц":
			showRecentFactsByEmployee(db)

		case "Добавить сотрудника":
			addEmployeePrompt(db)

		case "Добавить факт":
			addFact(db)

		case "Просмотр фактов":
			getFacts(db)

		case "Просмотр сотрудников":
			showEmployees(db)

		case "Показать рейтинги сотрудников":
			showEmployeeRatings(db)

		case "Удалить все факты":
			deleteAllFacts(db)

		case "Удалить всех сотрудников":
			deleteAllEmployees(db)

		case "Топ 3 сотрудника по положительному рейтингу":
			showTopEmployees(db, "положительная")

		case "Топ 3 сотрудника с отрицательным рейтингом":
			showTopEmployees(db, "отрицательная")

		case "Выход":
			return
		}
	}
}
