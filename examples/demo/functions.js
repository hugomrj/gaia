// functions.js
// Catálogo de funciones del cliente.
// Agregá funciones nuevas acá. El chat solo responde con lo que está en este objeto.
//
// Estructura de cada función:
//   nombre_de_funcion: {
//     description: 'Qué hace en una línea.',
//     required: ['param_obligatorio'],
//     optional: { param_opcional: valorDefault },
//     execute(p){ return 'texto crudo para el synthesizer'; }
//   },
//
// Disponibles globalmente (definidas en index.html):
//   DB, gs(), fmtNum(), sumBy(), pct(), norm(), iso(), isoFrom(), todayISO(),
//   diffDays(), monthRange(), weekRange(), yearRange(), prevMonthRange(),
//   prevWeekRange(), resolveRange(), inRange(), expensesInRange(),
//   findCustomer(), findProduct(), findSupplier(), findEmployee(), nextOC()

const CATALOG = {

  /* ---------- 💼 FACTURACIÓN Y VENTAS ---------- */

get_daily_billing: {
  description: 'Total facturado en una fecha (día).',
  required: ['date'], optional: {},
  execute(p){
    let d = p.date;
    if(d && typeof d === 'object') d = d.from ?? d.to;
    if(typeof d !== 'string') throw new Error('fecha inválida: ' + JSON.stringify(d));
    const t = norm(d);
    let date = d;
    if(t === 'hoy') date = todayISO();
    else if(t === 'ayer') date = iso(-1);
    const m = date.match(/\d{4}-\d{2}-\d{2}/);
    if(!m) throw new Error('no pude interpretar la fecha: ' + d);
    const total = DB.invoices.filter(f => f.date === m[0]).reduce((s,f)=>s+f.amount, 0);
    return `Facturación del día ${m[0]}: ${gs(total)}`;
  }
},

get_period_billing: {
  description: 'Total facturado en un período (ej: este mes, esta semana, año 2026).',
  required: ['period'], optional: {},
  execute(p){
    const period = String(p.period ?? p.date ?? '').trim();

    // Caso: año específico ("2026" o 2026)
    if(/^\d{4}$/.test(period)){
      const r = { from: `${period}-01-01`, to: `${period}-12-31` };
      const total = sumBy(inRange(r), f => f.amount);
      return `Facturación del año ${period}: ${gs(total)}`;
    }

    const r = resolveRange(p) || resolveRange({ period });
    if(!r) throw new Error('no pude interpretar el período: ' + JSON.stringify(p));
    const total = sumBy(inRange(r), f => f.amount);
    return `Facturación del período ${r.from} a ${r.to}: ${gs(total)}`;
  }
},


compare_billing_periods: {
  description: 'Compara la facturación de un período contra el anterior (% de crecimiento).',
  required: [], optional: { period: null },
  execute(p){
    const r = resolveRange(p) || monthRange();
    const kind = norm(p?.period || 'mes');

    // Detectar el tipo y calcular el período anterior correctamente
    let prev;
    if(kind.includes('mes')){
      // Mes anterior: mismo día 1 al último día del mes anterior
      const [y, m] = r.from.split('-').map(Number);
      const prevY = m === 1 ? y - 1 : y;
      const prevM = m === 1 ? 12 : m - 1;
      const first = new Date(prevY, prevM - 1, 1);
      const last = new Date(prevY, prevM, 0);
      prev = { from: fmtISO(first), to: fmtISO(last) };
    } else if(kind.includes('año') || kind.includes('ano')){
      // Año anterior: 1 de enero al 31 de diciembre
      const y = parseInt(r.from.slice(0, 4)) - 1;
      prev = { from: `${y}-01-01`, to: `${y}-12-31` };
    } else if(kind.includes('semana')){
      // Semana anterior
      prev = { from: isoFrom(r.from, -7), to: isoFrom(r.to, -7) };
    } else {
      // Fallback: mismo largo de días
      const days = diffDays(r.from, r.to) + 1;
      prev = { from: isoFrom(r.from, -days), to: isoFrom(r.to, -days) };
    }

    const cur = sumBy(inRange(r), f=>f.amount);
    const ant = sumBy(inRange(prev), f=>f.amount);
    if(!ant) return `período actual (${r.from} a ${r.to}): ${gs(cur)} | sin datos del período anterior para calcular variación`;
    const g = (cur - ant) / ant * 100;
    return `período actual (${r.from} a ${r.to}): ${gs(cur)} | anterior (${prev.from} a ${prev.to}): ${gs(ant)} | variación: ${g>=0?'+':''}${g.toFixed(1)}%`;
  }
},

get_billing_by_month: {
  description: 'Facturación desglosada por mes (últimos N meses).',
  required: [], optional: { months: 6 },
  execute(p){
    const n = Math.min(Math.max(parseInt(p?.months) || 6, 1), 12);
    const now = new Date();
    const rows = [];
    let total = 0;
    for(let i = n - 1; i >= 0; i--){
      const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
      const first = new Date(d.getFullYear(), d.getMonth(), 1);
      const last = new Date(d.getFullYear(), d.getMonth() + 1, 0);
      const r = { from: fmtISO(first), to: fmtISO(last) };
      const m = sumBy(inRange(r), f=>f.amount);
      total += m;
      const label = d.toLocaleDateString('es-PY', { month: 'long', year: 'numeric' });
      rows.push(`• ${label}: ${gs(m)}`);
    }
    return `Facturación por mes (últimos ${n} meses):\n` + rows.join('\n') + `\nTOTAL: ${gs(total)}`;
  }
},

  get_top_customers: {
    description: 'Clientes con mayor facturación en un período.',
    required: [], optional: { period: null, limit: 5 },
    execute(p){
      const r = resolveRange(p) || monthRange();
      const limit = Math.min(Math.max(parseInt(p?.limit) || 5, 1), 15);
      const list = inRange(r);
      if(!list.length) return `Sin facturación entre ${r.from} y ${r.to}`;
      const by = {};
      list.forEach(f => by[f.customer] = (by[f.customer]||0) + f.amount);
      const rows = Object.entries(by).sort((a,b)=>b[1]-a[1]).slice(0,limit);
      return `top clientes por facturación (${r.from} a ${r.to})\n` + rows.map(([n,v],i)=>`${i+1}. ${n}: ${gs(v)}`).join('\n');
    }
  },

  get_top_products: {
    description: 'Productos más vendidos en un período (default: mes).',
    required: [], optional: { limit: 5, period: null },
    execute(p){
      const limit = Math.min(Math.max(parseInt(p?.limit) || 5, 1), 20);
      // Tu DB no tiene sold_by_period, solo sold_month.
      // Si te piden "hoy", avisá que solo tenés datos del mes.
      const period = p?.period ? norm(p.period) : 'mes';
      if(period !== 'mes' && period !== 'del mes'){
        return `Solo tengo ranking de productos del MES, no por día. Los más vendidos del mes son: ` +
          [...DB.products].sort((a,b)=>b.sold_month-a.sold_month).slice(0,limit)
            .map((x,i)=>`${i+1}. ${x.name} (${x.sold_month} u)`).join('; ');
      }
      return [...DB.products].sort((a,b)=>b.sold_month-a.sold_month).slice(0,limit)
        .map((x,i)=>`${i+1}. ${x.name} (${x.sold_month} u)`).join('; ');
    }
  },

  generate_sales_report: {
    description: 'Informe gerencial de ventas de un período (total, cobrado, margen, ticket promedio, top clientes).',
    required: ['period'], optional: {},
    execute(p){
      const r = resolveRange(p) || resolveRange({ period: p.period ?? p.date });
      if(!r) throw new Error('no pude interpretar el período: ' + JSON.stringify(p));
      const list = inRange(r);
      if(!list.length) return `Sin ventas entre ${r.from} y ${r.to}`;
      const total = sumBy(list,f=>f.amount);
      const cost  = sumBy(list,f=>f.cost);
      const paid  = sumBy(list.filter(f=>f.status==='paid'), f=>f.amount);
      const byCust = {};
      list.forEach(f => byCust[f.customer] = (byCust[f.customer]||0) + f.amount);
      const top = Object.entries(byCust).sort((a,b)=>b[1]-a[1]).slice(0,3).map(([n,v])=>`${n} (${fmtNum(v)})`).join(', ');
      return `informe de ventas ${r.from} a ${r.to}\nFacturas: ${list.length}\nTotal facturado: ${gs(total)}\nCobrado: ${gs(paid)} (${pct(paid,total)}%)\nMargen bruto: ${gs(total-cost)} (${pct(total-cost,total)}%)\nTicket promedio: ${gs(Math.round(total/list.length))}\nTop clientes: ${top}`;
    }
  },

  /* ---------- 📈 RENTABILIDAD ---------- */

  get_profit_margin: {
    description: 'Margen de ganancia bruta de un período (ingresos vs costos).',
    required: [], optional: { period: null },
    execute(p){
      const r = resolveRange(p) || monthRange();
      const list = inRange(r);
      const rev = sumBy(list, f=>f.amount);
      const cost = sumBy(list, f=>f.cost);
      return `margen del período ${r.from} a ${r.to} | ingresos ${gs(rev)} | costos ${gs(cost)} | ganancia bruta ${gs(rev-cost)} | margen ${pct(rev-cost,rev)}%`;
    }
  },

  get_top_margin_products: {
    description: 'Productos con mayor margen unitario (precio vs costo).',
    required: [], optional: { limit: 5 },
    execute(p){
      const limit = Math.min(Math.max(parseInt(p?.limit) || 5, 1), 15);
      const rows = DB.products
        .map(x => ({ name: x.name, unit: x.price - x.cost, mp: (x.price - x.cost) / x.price * 100 }))
        .sort((a,b)=>b.mp-a.mp).slice(0,limit);
      return 'productos con mayor margen unitario\n' + rows.map((x,i)=>`${i+1}. ${x.name}: ${x.mp.toFixed(1)}% (${gs(x.unit)} por unidad)`).join('\n');
    }
  },

  get_pnl_summary: {
    description: 'Estado de resultados resumido del período (P&L: ingresos, costos, gastos, resultado neto).',
    required: [], optional: { period: null },
    execute(p){
      const r = resolveRange(p) || monthRange();
      const list = inRange(r);
      const rev = sumBy(list, f=>f.amount);
      const cogs = sumBy(list, f=>f.cost);
      const gross = rev - cogs;
      const expT = sumBy(expensesInRange(r), e=>e.amount);
      const net = gross - expT;
      return `P&L ${r.from} a ${r.to}\nIngresos: ${gs(rev)}\nCosto de ventas: ${gs(cogs)}\nUtilidad bruta: ${gs(gross)} (${pct(gross,rev)}%)\nGastos operativos: ${gs(expT)}\nResultado neto: ${gs(net)}`;
    }
  },

  /* ---------- 💸 GASTOS Y PRESUPUESTO ---------- */

  get_expense_total: {
    description: 'Total de gastos operativos de un período.',
    required: [], optional: { period: null },
    execute(p){
      const r = resolveRange(p) || monthRange();
      const list = expensesInRange(r);
      return `gastos totales ${r.from} a ${r.to}: ${gs(sumBy(list,e=>e.amount))} (${list.length} registros)`;
    }
  },

  get_expenses_by_category: {
    description: 'Gastos agrupados por categoría en un período.',
    required: [], optional: { period: null },
    execute(p){
      const r = resolveRange(p) || monthRange();
      const list = expensesInRange(r);
      if(!list.length) return `Sin gastos registrados entre ${r.from} y ${r.to}`;
      const by = {};
      list.forEach(e => by[e.category] = (by[e.category]||0) + e.amount);
      return `gastos por categoría (${r.from} a ${r.to})\n` +
        Object.entries(by).sort((a,b)=>b[1]-a[1]).map(([c,v])=>`${c}: ${gs(v)}`).join('\n') +
        `\nTOTAL: ${gs(sumBy(list,e=>e.amount))}`;
    }
  },

  get_budget_vs_actual: {
    description: 'Presupuesto vs real del mes (ingresos contra meta y gastos por categoría).',
    required: [], optional: {},
    execute(){
      const r = monthRange();
      const rev = sumBy(inRange(r), f=>f.amount);
      const target = DB.budget?.month_revenue_target || 0;
      const expBy = {};
      expensesInRange(r).forEach(e => expBy[e.category] = (expBy[e.category]||0) + e.amount);
      const bud = DB.budget?.expenses || {};
      const rows = Object.entries(bud).map(([c,b])=>{
        const a = expBy[c] || 0;
        return `${c}: real ${gs(a)} / presupuestado ${gs(b)} (${a<=b ? 'en meta' : 'excedido ' + pct(a-b,b) + '%'})`;
      });
      return `presupuesto vs real (${r.from} a ${r.to})\nIngresos: ${gs(rev)} / meta ${gs(target)} → ${pct(rev,target)}%\n` + rows.join('\n');
    }
  },

  /* ---------- 💰 COBRANZAS Y CAJA ---------- */

  list_overdue_invoices: {
    description: 'Facturas vencidas impagas (opcionalmente de un cliente).',
    required: [], optional: { customer: null },
    execute(p){
      const t = todayISO();
      let list = DB.invoices.filter(f => f.status === 'pending' && f.due_date < t);
      if(p?.customer){
        const q = norm(p.customer);
        list = list.filter(f => norm(f.customer).includes(q));
      }
      if(!list.length) return 'Sin facturas vencidas';
      return list.map(f => `${f.id} | ${f.customer} | ${gs(f.amount)} | venció ${f.due_date}`).join('\n');
    }
  },

  get_receivables_summary: {
    description: 'Resumen de cuentas por cobrar con antigüedad de deuda (aging).',
    required: [], optional: {},
    execute(){
      const t = todayISO();
      const pend = DB.invoices.filter(f => f.status === 'pending');
      const notDue = pend.filter(f => f.due_date >= t);
      const late = pend.filter(f => f.due_date < t);
      const b30  = late.filter(f => diffDays(f.due_date, t) <= 30);
      const b60  = late.filter(f => diffDays(f.due_date, t) > 30 && diffDays(f.due_date, t) <= 60);
      const b60p = late.filter(f => diffDays(f.due_date, t) > 60);
      return `cuentas por cobrar: total ${gs(sumBy(pend,f=>f.amount))} (${pend.length} facturas)\nPor vencer: ${gs(sumBy(notDue,f=>f.amount))}\nVencidas 1-30 días: ${gs(sumBy(b30,f=>f.amount))} (${b30.length})\nVencidas 31-60 días: ${gs(sumBy(b60,f=>f.amount))} (${b60.length})\nVencidas +60 días: ${gs(sumBy(b60p,f=>f.amount))} (${b60p.length})`;
    }
  },

  get_collection_rate: {
    description: 'Tasa de cobranza de un período (cobrado vs facturado).',
    required: [], optional: { period: null },
    execute(p){
      const r = resolveRange(p) || monthRange();
      const list = inRange(r);
      const tot = sumBy(list, f=>f.amount);
      const paid = sumBy(list.filter(f=>f.status==='paid'), f=>f.amount);
      return `cobranza ${r.from} a ${r.to} | facturado ${gs(tot)} | cobrado ${gs(paid)} | tasa ${pct(paid,tot)}% | pendiente ${gs(tot-paid)}`;
    }
  },

  get_cash_position: {
    description: 'Posición de caja consolidada (saldos de cuentas bancarias).',
    required: [], optional: {},
    execute(){
      const total = sumBy(DB.bank_accounts, b=>b.balance);
      return DB.bank_accounts.map(b => `${b.bank} (${b.account}): ${gs(b.balance)}`).join('\n') + `\nTOTAL: ${gs(total)}`;
    }
  },

  /* ---------- 📦 INVENTARIO Y COMPRAS ---------- */

  check_stock: {
    description: 'Stock disponible de un producto.',
    required: ['product'], optional: {},
    execute(p){
      const prod = findProduct(p.product);
      if(!prod) return `Sin resultados para «${p.product}»`;
      return `${prod.sku} | ${prod.name} | stock ${prod.stock} u (mínimo ${prod.min_stock} u) | costo ${fmtNum(prod.cost)} | precio ${fmtNum(prod.price)} | ${prod.warehouse}`;
    }
  },

  list_low_stock: {
    description: 'Productos con stock bajo el mínimo, con cantidad sugerida de compra.',
    required: [], optional: {},
    execute(){
      const low = DB.products.filter(x => x.stock <= x.min_stock);
      if(!low.length) return 'Sin productos bajo el mínimo';
      return 'productos bajo mínimo\n' + low.map(x => `${x.name}: ${x.stock} u (mín ${x.min_stock} u) → sugerido comprar ${Math.max(x.min_stock*2 - x.stock, 1)} u`).join('\n');
    }
  },

  get_inventory_value: {
    description: 'Valorización del inventario (a costo y a precio de venta).',
    required: [], optional: {},
    execute(){
      const costV = sumBy(DB.products, x=>x.stock*x.cost);
      const saleV = sumBy(DB.products, x=>x.stock*x.price);
      const units = sumBy(DB.products, x=>x.stock);
      const low = DB.products.filter(x => x.stock <= x.min_stock);
      return `inventario: ${DB.products.length} productos | ${units} unidades | valor a costo ${gs(costV)} | valor a venta ${gs(saleV)} | bajo mínimo: ${low.length ? low.map(x=>x.name).join(', ') : 'ninguno'}`;
    }
  },

  create_purchase_order: {
    description: 'Crea una orden de compra a un proveedor.',
    required: ['supplier', 'product', 'quantity'], optional: {},
    execute(p){
      const qty = parseInt(p.quantity);
      if(!isFinite(qty) || qty <= 0) throw new Error('cantidad inválida: ' + p.quantity);
      const prod = findProduct(p.product);
      if(!prod) throw new Error('producto no encontrado: ' + p.product);
      const sup = findSupplier(p.supplier);
      const oc = { id: nextOC(), supplier: sup ? sup.name : String(p.supplier), product: prod.name, quantity: qty, total: qty * prod.cost, status: 'pendiente', expected: iso(7) };
      DB.purchase_orders.push(oc);
      return `OK ${oc.id} | ${oc.supplier} | ${oc.product} x${qty} | total ${gs(oc.total)} | entrega estimada ${oc.expected}`;
    }
  },

  list_purchase_orders: {
    description: 'Órdenes de compra (por defecto las pendientes).',
    required: [], optional: { status: 'pendiente' },
    execute(p){
      const st = p?.status ? norm(p.status) : 'pendiente';
      const list = st === 'todas' ? DB.purchase_orders : DB.purchase_orders.filter(o => norm(o.status) === st);
      if(!list.length) return `Sin órdenes de compra (${st})`;
      return list.map(o => `${o.id} | ${o.supplier} | ${o.product} x${o.quantity} | ${gs(o.total)} | ${o.status} | entrega ${o.expected}`).join('\n');
    }
  },

  /* ---------- 👥 CLIENTES Y RRHH ---------- */

  search_customer: {
    description: 'Ficha de un cliente (vendedor, ciudad, deuda, crédito).',
    required: ['name'], optional: {},
    execute(p){
      const c = findCustomer(p.name);
      if(!c) return `Sin resultados para «${p.name}»`;
      return `${c.id} | ${c.name} | ${c.segment} · ${c.city} | vendedor: ${c.seller} | tel ${c.phone} | deuda ${gs(c.debt)} | crédito ${gs(c.credit_limit)}`;
    }
  },

  check_credit_limit: {
    description: 'Uso del límite de crédito de un cliente (deuda vs disponible).',
    required: ['customer'], optional: {},
    execute(p){
      const c = findCustomer(p.customer);
      if(!c) return `Sin resultados para «${p.customer}»`;
      return `${c.name} | deuda ${gs(c.debt)} | límite ${gs(c.credit_limit)} | disponible ${gs(c.credit_limit - c.debt)} | uso del crédito ${pct(c.debt, c.credit_limit)}%`;
    }
  },

  search_employee: {
    description: 'Datos de un empleado (cargo, departamento, email, sueldo).',
    required: ['name'], optional: {},
    execute(p){
      const e = findEmployee(p.name);
      if(!e) return `Sin resultados para «${p.name}»`;
      return `${e.id} | ${e.name} | ${e.position} | ${e.department} | ${e.email} | sueldo ${gs(e.salary)}`;
    }
  },

  get_department_headcount: {
    description: 'Dotación y nómina por departamento.',
    required: [], optional: {},
    execute(){
      const by = {};
      DB.employees.forEach(e => (by[e.department] = by[e.department] || []).push(e));
      return Object.entries(by).map(([d,l]) => `${d}: ${l.length} personas (${gs(sumBy(l,e=>e.salary))} de nómina)`).join('\n');
    }
  },

  get_seller_performance: {
    description: 'Desempeño de un vendedor en el mes (facturación, ticket promedio, clientes).',
    required: ['seller'], optional: {},
    execute(p){
      const q = norm(p.seller);
      const emp = findEmployee(p.seller);
      const name = emp ? emp.name : p.seller;
      const r = monthRange();
      const list = inRange(r).filter(f => norm(f.seller).includes(q) || (emp && f.seller === name));
      if(!list.length) return `Sin ventas de ${name} en el mes actual (${r.from} a ${r.to})`;
      const total = sumBy(list, f=>f.amount);
      const clients = new Set(list.map(f=>f.customer)).size;
      return `${name} (mes ${r.from} a ${r.to}) | facturas: ${list.length} | total ${gs(total)} | ticket promedio ${gs(Math.round(total/list.length))} | clientes atendidos: ${clients}`;
    }
  },

  create_invoice: {
    description: 'Registra una nueva factura.',
    required: ['customer', 'amount'], optional: { date: null },
    execute(p){
      const amount = Number(String(p.amount).replace(/[^\d.]/g,''));
      if(!isFinite(amount) || amount <= 0) throw new Error('monto inválido: ' + p.amount);
      const c = findCustomer(p.customer);
      const custName = c ? c.name : String(p.customer);
      const date = (typeof p.date === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(p.date)) ? p.date : todayISO();
      const inv = { id:'FAC-'+String(DB._invoiceSeq++).padStart(4,'0'), customer:custName, date, amount:Math.round(amount), status:'pending', due_date: isoFrom(date, 15), seller: c?.seller || 'Mostrador', payment_method: 'crédito 30 días', cost: Math.round(amount * 0.75) };
      DB.invoices.push(inv);
      return `OK ${inv.id} | ${inv.customer} | ${gs(inv.amount)} | ${inv.date} | vence ${inv.due_date}`;
    }
  },

  schedule_meeting: {
    description: 'Agenda una reunión o visita comercial.',
    required: ['date', 'time', 'client'], optional: {},
    execute(p){
      const m = { id:'REA-'+String(DB.meetings.length+1).padStart(3,'0'), date:String(p.date), time:String(p.time), client:String(p.client) };
      DB.meetings.push(m);
      return `OK ${m.id} | ${m.client} | ${m.date} ${m.time}`;
    }
  },

  list_meetings: {
    description: 'Reuniones y visitas agendadas.',
    required: [], optional: {},
    execute(){
      if(!DB.meetings.length) return 'Sin reuniones agendadas';
      return DB.meetings.map(m => `${m.id} | ${m.client} | ${m.date} ${m.time}`).join('\n');
    }
  },

  /* ---------- 🏭 PROVEEDORES ---------- */

  search_supplier: {
    description: 'Ficha de un proveedor (rubro, deuda, contacto).',
    required: ['name'], optional: {},
    execute(p){
      const s = findSupplier(p.name);
      if(!s) return `Sin resultados para «${p.name}»`;
      return `${s.id} | ${s.name} | rubro ${s.category} | deuda ${gs(s.debt)} | contacto ${s.contact}`;
    }
  },

  get_suppliers_debt: {
    description: 'Deuda total con proveedores, desglosada.',
    required: [], optional: {},
    execute(){
      const total = sumBy(DB.suppliers, s=>s.debt);
      const withDebt = DB.suppliers.filter(s => s.debt > 0).sort((a,b)=>b.debt-a.debt);
      return `deuda total a proveedores: ${gs(total)}\n` + (withDebt.map(s => `${s.name}: ${gs(s.debt)}`).join('\n') || 'sin deudas');
    }
  },

  /* ---------- 📊 TABLERO GERENCIAL ---------- */

  get_kpi_dashboard: {
    description: 'Tablero gerencial consolidado del mes (facturación, margen, gastos, resultado, cobranzas, caja, inventario).',
    required: [], optional: {},
    execute(){
      const r = monthRange();
      const list = inRange(r);
      const rev = sumBy(list, f=>f.amount);
      const cost = sumBy(list, f=>f.cost);
      const gross = rev - cost;
      const expT = sumBy(expensesInRange(r), e=>e.amount);
      const net = gross - expT;
      const t = todayISO();
      const pend = DB.invoices.filter(f => f.status === 'pending');
      const over = pend.filter(f => f.due_date < t);
      const cash = sumBy(DB.bank_accounts, b=>b.balance);
      const invVal = sumBy(DB.products, x=>x.stock*x.cost);
      const low = DB.products.filter(x => x.stock <= x.min_stock).length;
      const bySeller = {};
      list.forEach(f => bySeller[f.seller] = (bySeller[f.seller]||0) + f.amount);
      const topSeller = Object.entries(bySeller).sort((a,b)=>b[1]-a[1])[0];
      return [
        `📊 KPIs del mes (${r.from} a ${r.to})`,
        `Facturación: ${gs(rev)} (meta ${gs(DB.budget?.month_revenue_target || 0)} → ${pct(rev, DB.budget?.month_revenue_target || 0)}%)`,
        `Margen bruto: ${gs(gross)} (${pct(gross,rev)}%)`,
        `Gastos operativos: ${gs(expT)}`,
        `Resultado neto: ${gs(net)}`,
        `Cuentas por cobrar: ${gs(sumBy(pend,f=>f.amount))} | vencidas: ${gs(sumBy(over,f=>f.amount))}`,
        `Posición de caja: ${gs(cash)}`,
        `Valor de inventario (costo): ${gs(invVal)} | ${low} productos bajo mínimo`,
        topSeller ? `Top vendedor: ${topSeller[0]} (${gs(topSeller[1])})` : 'Sin ventas registradas en el mes'
      ].join('\n');
    }
  },

  /* ---------- 🤖 META / CONVERSACIONALES ---------- */

  get_bot_info: {
    description: 'Información sobre el asistente (quién es, qué puede hacer).',
    required: [], optional: {},
    execute(){
      const total = Object.keys(CATALOG).length;
      return `Soy GAIA, el asistente gerencial de ${DB.company.name}. Puedo ejecutar ${total} funciones sobre el ERP: facturación, rentabilidad, gastos, cobranzas, caja, inventario, proveedores, RRHH y KPIs gerenciales.`;
    }
  },

  list_capabilities: {
    description: 'Lista las capacidades del asistente.',
    required: [], optional: {},
    execute(){
      return 'Puedo ayudarte con:\n' + Object.values(CATALOG).map(f => '• ' + f.description).join('\n');
    }
  },

  get_my_profile: {
    description: 'Perfil del usuario actual.',
    required: [], optional: {},
    execute(){
      return 'No tengo acceso a tu información personal en esta sesión. Si me das tu nombre o documento, puedo buscarte en el sistema.';
    }
  },

  get_greeting: {
    description: 'Saludo inicial.',
    required: [], optional: {},
    execute(){
      return `¡Hola! Soy el asistente gerencial de ${DB.company.name}. ¿En qué te puedo ayudar?`;
    }
  },

  get_farewell: {
    description: 'Despedida.',
    required: [], optional: {},
    execute(){
      return '¡Hasta pronto! Si necesitás algo más, acá estoy.';
    }
  },

  get_thanks: {
    description: 'Respuesta a agradecimiento.',
    required: [], optional: {},
    execute(){
      return '¡De nada! Estoy para lo que necesites.';
    }
  },

  get_help: {
    description: 'Ayuda general.',
    required: [], optional: {},
    execute(){
      return 'Podés preguntarme sobre facturación, rentabilidad, gastos, cobranzas, caja, inventario, proveedores, RRHH y KPIs. Probá una sugerencia o escribí tu consulta.';
    }
  },


  get_daily_sales_products: {
    description: 'Productos vendidos en un día, con cantidad por producto.',
    required: ['date'], optional: {},
    execute(p){
      let d = p.date;
      if(d && typeof d === 'object') d = d.from ?? d.to;
      if(typeof d !== 'string') throw new Error('fecha inválida: ' + JSON.stringify(d));
      const t = norm(d);
      let date = d;
      if(t === 'hoy') date = todayISO();
      else if(t === 'ayer') date = iso(-1);
      const m = date.match(/\d{4}-\d{2}-\d{2}/);
      if(!m) throw new Error('no pude interpretar la fecha: ' + d);

      const invoices = DB.invoices.filter(f => f.date === m[0]);
      if(!invoices.length) return `Sin ventas registradas el ${m[0]}`;

      // Agrupar items por SKU
      const bySku = {};
      invoices.forEach(f => (f.items || []).forEach(it => {
        bySku[it.sku] = (bySku[it.sku] || 0) + it.qty;
      }));

      const rows = Object.entries(bySku)
        .map(([sku, qty]) => {
          const prod = DB.products.find(x => x.sku === sku);
          return { name: prod?.name || sku, qty };
        })
        .sort((a,b) => b.qty - a.qty);

      if(!rows.length) return `${invoices.length} facturas pero sin detalle de items el ${m[0]}`;

      const totalUnits = rows.reduce((s,x) => s + x.qty, 0);
      return `Productos vendidos el ${m[0]}:\n` +
        rows.map(r => `• ${r.name}: ${r.qty} u`).join('\n') +
        `\nTotal: ${totalUnits} unidades en ${invoices.length} facturas`;
    }
  },


get_accumulated_billing: {
  description: 'Facturación total acumulada (año actual o histórica).',
  required: [], optional: { period: 'año' },
  execute(p){
    const kind = norm(p?.period || 'año');
    const isYear = kind.includes('año') || kind.includes('ano') || kind.includes('año actual');
    const r = isYear ? yearRange() : { from: '2000-01-01', to: '2999-12-31' };
    const list = inRange(r);
    const total = sumBy(list, f=>f.amount);
    const label = isYear ? `año ${new Date().getFullYear()}` : 'histórico';
    return `Facturación acumulada (${label}): ${gs(total)} en ${list.length} facturas`;
  }
},



};

window.CATALOG = CATALOG;