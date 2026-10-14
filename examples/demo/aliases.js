// aliases.js
// Normalizador de nombres de funciones.
//
// El Planner puede inventar variantes que no existen literalmente en CATALOG.
// Este mapa las redirige a las funciones reales.
//
// Agregá entradas cuando veas "function_not_found" pero la función SÍ existe
// en tu catálogo con otro nombre.

const FUNCTION_ALIASES = {

  // ---------- Crecimiento / comparaciones ----------
  "calculate_monthly_growth":   "compare_billing_periods",
  "calculate_growth":           "compare_billing_periods",
  "calculate_growth_rate":      "compare_billing_periods",
  "calculate_period_comparison":"compare_billing_periods",
  "compare_revenue":            "compare_billing_periods",
  "compare_sales":              "compare_billing_periods",
  "compare_periods":            "compare_billing_periods",
  "get_revenue_growth":         "compare_billing_periods",
  "query_monthly_growth":       "compare_billing_periods",
  "query_growth":               "compare_billing_periods",
  "get_billing_comparison":     "compare_billing_periods",
  "compare_billing":            "compare_billing_periods",

  // ---------- Facturación — variantes ----------
  "get_billing":                "get_period_billing",
  "query_billing":              "get_period_billing",
  "get_total_billing":          "get_period_billing",
  "calculate_billing":          "get_period_billing",
  "get_billing_total":          "get_period_billing",
  "get_sales":                  "get_period_billing",
  "get_revenue":                "get_period_billing",
  "query_sales":                "get_period_billing",
  "query_revenue":              "get_period_billing",
  "get_period_sales":           "get_period_billing",



    // ---------- Facturación por período ----------
  "get_monthly_billing":        "get_period_billing",
  "get_daily_billing_summary":  "get_period_billing",
  "query_monthly_billing":      "get_period_billing",
  "query_period_billing":       "get_period_billing",
  "get_billing_for_period":     "get_period_billing",
  "get_billing_by_period":      "get_period_billing",
  "get_weekly_billing":         "get_period_billing",
  "get_yearly_billing":         "get_period_billing",
  "query_weekly_billing":       "get_period_billing",
  "query_yearly_billing":       "get_period_billing",
  "get_month_billing":          "get_period_billing",
  "get_current_month_billing":  "get_period_billing",
  "calculate_period_billing":   "get_period_billing",
  "get_billing_this_month":     "get_period_billing",


  

  // ---------- Facturación diaria ----------
  "get_today_billing":          "get_daily_billing",
  "get_billing_today":          "get_daily_billing",
  "query_daily_billing":        "get_daily_billing",
  "get_daily_sales":            "get_daily_billing",
  "get_sales_today":            "get_daily_billing",

  // ---------- Ventas por vendedor ----------
  "get_sales_by_seller":        "get_billing_by_seller",
  "search_sales_by_seller":     "get_billing_by_seller",
  "search_billing_by_seller":   "get_billing_by_seller",
  "list_sales_by_seller":       "get_billing_by_seller",
  "list_billing_by_seller":     "get_billing_by_seller",
  "get_seller_billing":         "get_billing_by_seller",
  "get_seller_sales":           "get_billing_by_seller",
  "sales_by_seller":            "get_billing_by_seller",
  "get_sales_by_vendor":        "get_billing_by_seller",
  "sales_by_vendor":            "get_billing_by_seller",
  "query_sales_by_seller":      "get_billing_by_seller",
  "query_seller_sales":         "get_billing_by_seller",

  // ---------- Top clientes / productos ----------
  "get_best_customers":         "get_top_customers",
  "get_best_clients":           "get_top_customers",
  "list_top_customers":         "get_top_customers",
  "get_customer_ranking":       "get_top_customers",
  "get_best_products":          "get_top_products",
  "list_top_products":          "get_top_products",
  "get_product_ranking":        "get_top_products",

  // ---------- Rentabilidad ----------
  "get_margin":                 "get_profit_margin",
  "get_margin_profit":          "get_profit_margin",
  "calculate_margin":           "get_profit_margin",
  "calculate_profit_margin":    "get_profit_margin",
  "get_profit":                 "get_profit_margin",
  "get_top_margin":             "get_top_margin_products",
  "get_best_margin_products":   "get_top_margin_products",
  "get_pnl":                    "get_pnl_summary",
  "get_profit_loss":            "get_pnl_summary",
  "get_income_statement":       "get_pnl_summary",

  // ---------- Gastos ----------
  "get_expenses":               "get_expense_total",
  "get_total_expenses":         "get_expense_total",
  "query_expenses":             "get_expense_total",
  "get_expense_by_category":    "get_expenses_by_category",
  "get_expenses_categories":    "get_expenses_by_category",
  "list_expense_categories":    "get_expenses_by_category",
  "get_budget":                 "get_budget_vs_actual",
  "compare_budget":             "get_budget_vs_actual",
  "get_budget_status":          "get_budget_vs_actual",

  // ---------- Cobranzas ----------
  "get_overdue_invoices":       "list_overdue_invoices",
  "get_pending_invoices":       "list_overdue_invoices",
  "list_pending_invoices":      "list_overdue_invoices",
  "get_unpaid_invoices":        "list_overdue_invoices",
  "list_unpaid_invoices":       "list_overdue_invoices",
  "get_accounts_receivable":    "get_receivables_summary",
  "get_ar_summary":             "get_receivables_summary",
  "get_ar":                     "get_receivables_summary",
  "get_receivables":            "get_receivables_summary",
  "get_collection":             "get_collection_rate",
  "get_collection_summary":     "get_collection_rate",

  // ---------- Caja ----------
  "get_cash":                   "get_cash_position",
  "get_bank_balance":           "get_cash_position",
  "get_bank_accounts":          "get_cash_position",
  "get_balances":               "get_cash_position",
  "get_liquidity":              "get_cash_position",

  // ---------- Inventario ----------
  "get_stock":                  "check_stock",
  "check_inventory":            "check_stock",
  "query_stock":                "check_stock",
  "get_product_stock":          "check_stock",
  "get_low_stock":              "list_low_stock",
  "list_low_stock_products":    "list_low_stock",
  "get_stock_alerts":           "list_low_stock",
  "get_products_low_stock":     "list_low_stock",
  "get_inventory":              "get_inventory_value",
  "get_inventory_valuation":    "get_inventory_value",
  "calculate_inventory_value":  "get_inventory_value",
  "get_stock_value":            "get_inventory_value",

  // ---------- Compras ----------
  "create_order":               "create_purchase_order",
  "create_purchase":            "create_purchase_order",
  "make_purchase_order":        "create_purchase_order",
  "get_purchase_orders":        "list_purchase_orders",
  "list_orders":                "list_purchase_orders",
  "get_orders":                 "list_purchase_orders",

  // ---------- Clientes ----------
  "get_customer":               "search_customer",
  "get_client":                 "search_customer",
  "find_customer":              "search_customer",
  "find_client":                "search_customer",
  "get_client_info":            "search_customer",
  "get_credit":                 "check_credit_limit",
  "check_credit":               "check_credit_limit",
  "get_client_credit":          "check_credit_limit",

  // ---------- RRHH ----------
  "get_employee":               "search_employee",
  "find_employee":              "search_employee",
  "get_employee_info":          "search_employee",
  "get_headcount":              "get_department_headcount",
  "get_departments":            "get_department_headcount",
  "get_staff":                  "get_department_headcount",
  "get_seller_stats":           "get_seller_performance",
  "get_seller_sales_stats":     "get_seller_performance",

  // ---------- Proveedores ----------
  "get_supplier":               "search_supplier",
  "find_supplier":              "search_supplier",
  "get_suppliers":              "get_suppliers_debt",
  "get_payables":               "get_suppliers_debt",
  "get_accounts_payable":       "get_suppliers_debt",
  "get_ap":                     "get_suppliers_debt",
  "get_supplier_debt":          "get_suppliers_debt",

  // ---------- KPIs / Tablero ----------
  "calculate_monthly_kpis":     "get_kpi_dashboard",
  "calculate_kpis":             "get_kpi_dashboard",
  "get_monthly_kpis":           "get_kpi_dashboard",
  "query_kpis":                 "get_kpi_dashboard",
  "get_kpis_dashboard":         "get_kpi_dashboard",
  "get_dashboard_kpis":         "get_kpi_dashboard",
  "calculate_dashboard":        "get_kpi_dashboard",
  "get_monthly_dashboard":      "get_kpi_dashboard",
  "calculate_monthly_metrics":  "get_kpi_dashboard",
  "get_monthly_metrics":        "get_kpi_dashboard",
  "monthly_kpis":               "get_kpi_dashboard",
  "calculate_metrics":          "get_kpi_dashboard",
  "get_kpi":                    "get_kpi_dashboard",
  "get_kpis":                   "get_kpi_dashboard",
  "get_dashboard":              "get_kpi_dashboard",
  "get_summary":                "get_kpi_dashboard",
  "get_business_summary":       "get_kpi_dashboard",
  "get_overview":               "get_kpi_dashboard",

  // ---------- Meta / conversacionales ----------
  "query_bot_info":             "get_bot_info",
  "who_are_you":                "get_bot_info",
  "get_identity":               "get_bot_info",
  "query_my_profile":           "get_my_profile",
  "get_user_profile":           "get_my_profile",
  "get_my_info":                "get_my_profile",
  "query_my_info":              "get_my_profile",
  "get_capabilities":           "list_capabilities",
  "query_capabilities":         "list_capabilities",
  "get_capability_list":        "list_capabilities",
  "get_saludo":                 "get_greeting",
  "saludo":                     "get_greeting",
  "greet":                      "get_greeting",
  "get_despedida":              "get_farewell",
  "despedida":                  "get_farewell",
  "farewell":                   "get_farewell",


    // ---------- Top productos ----------
  "get_top_sold_products":      "get_top_products",
  "list_top_sold_products":     "get_top_products",
  "get_best_selling_products":  "get_top_products",
  "get_most_sold_products":     "get_top_products",
  "list_best_selling_products": "get_top_products",
  "query_top_products":         "get_top_products",
  "query_best_products":        "get_top_products",
  "get_product_ranking":        "get_top_products",
  "list_product_ranking":       "get_top_products",


  // aliases.js
"get_daily_sold_products":     "get_daily_sales_products",
"get_products_sold_today":     "get_daily_sales_products",
"list_daily_sold_products":    "get_daily_sales_products",
"query_daily_sales_products":  "get_daily_sales_products",
"get_sold_products":           "get_daily_sales_products",
"get_daily_product_sales":     "get_daily_sales_products",


  // ---------- Comparaciones de períodos ----------
  "calculate_comparison":       "compare_billing_periods",
  "compare_periods":            "compare_billing_periods",
  "compare_months":             "compare_billing_periods",
  "calculate_period_comparison":"compare_billing_periods",
  "get_period_comparison":      "compare_billing_periods",
  "query_period_comparison":    "compare_billing_periods",
  "compare_billing":            "compare_billing_periods",
  "compare_revenue":            "compare_billing_periods",
  "compare_sales":              "compare_billing_periods",
  "get_growth":                 "compare_billing_periods",
  "calculate_growth":           "compare_billing_periods",
  "get_growth_rate":            "compare_billing_periods",
  "calculate_growth_rate":      "compare_billing_periods",
  "get_monthly_comparison":     "compare_billing_periods",
  "compare_current_vs_previous":"compare_billing_periods",
  "get_comparison":             "compare_billing_periods",
  "query_comparison":           "compare_billing_periods",

// ---------- Breakdown por mes ----------
"list_totals_by_month":       "get_billing_by_month",
"get_monthly_totals":         "get_billing_by_month",
"list_monthly_totals":        "get_billing_by_month",
"get_totals_by_month":        "get_billing_by_month",
"get_month_by_month":         "get_billing_by_month",
"query_monthly_breakdown":    "get_billing_by_month",
"list_period_billing":        "get_billing_by_month",
"calculate_monthly_totals":   "get_billing_by_month",
"sum_by_month":               "get_billing_by_month",
"get_monthly_breakdown":      "get_billing_by_month",


// ---------- Facturación acumulada ----------
"get_accumulated_total":     "get_accumulated_billing",
"query_accumulated_total":   "get_accumulated_billing",
"get_total_accumulated":     "get_accumulated_billing",
"query_total_accumulated":   "get_accumulated_billing",
"get_accumulated_billing":   "get_accumulated_billing",
"get_total_billing_all":     "get_accumulated_billing",
"get_historical_billing":    "get_accumulated_billing",
"get_billing_total_all":     "get_accumulated_billing",


};

window.FUNCTION_ALIASES = FUNCTION_ALIASES;