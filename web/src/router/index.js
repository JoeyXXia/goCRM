import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    redirect: '/login'
  },
  {
    path: '/init',
    name: 'Init',
    component: () => import('@/view/init/index.vue')
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/view/login/index.vue')
  },
  {
    path: '/dealflow',
    name: 'DealflowWorkspace',
    component: () => import('@/view/dealflow/workspace.vue'),
    meta: { title: 'DealFlow 工作台', dealflow: true },
    children: [
      {
        path: 'customers',
        name: 'DealflowCustomers',
        component: () => import('@/view/dealflow/customers.vue'),
        meta: { title: '客户', customers: true }
      },
      {
        path: 'customers/:id([1-9][0-9]*)',
        name: 'DealflowCustomer',
        component: () => import('@/view/dealflow/customer.vue'),
        meta: { title: '客户详情', customers: true }
      },
      {
        path: 'departments/:id([1-9][0-9]*)',
        name: 'DealflowDepartment',
        component: () => import('@/view/dealflow/department.vue'),
        meta: { title: '部门详情' }
      },
      {
        path: 'departments/:id([1-9][0-9]*)/rename',
        name: 'DealflowRenameDepartment',
        component: () => import('@/view/dealflow/department.vue'),
        meta: { title: '修改部门名称', renameDepartment: true }
      }
    ]
  },
  {
    path: '/scanUpload',
    name: 'ScanUpload',
    meta: {
      title: '扫码上传',
      client: true
    },
    component: () => import('@/view/example/upload/scanUpload.vue')
  },
  {
    path: '/:catchAll(.*)',
    meta: {
      closeTab: true
    },
    component: () => import('@/view/error/index.vue')
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
