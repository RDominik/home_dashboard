import React from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import App from './App.jsx'
import DashboardHome from './pages/DashboardHome.jsx'
import Wallbox from './pages/Wallbox.jsx'
import Inverter from './pages/Inverter.jsx'
import EnergyFlow from './pages/EnergyFlow.jsx'
import Heating from './pages/Heating.jsx'
import Grafana from './pages/Grafana.jsx'
import UpdatePage from './pages/UpdatePage.jsx'
import GoE from './pages/goE.jsx'

const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    children: [
      { index: true, element: <DashboardHome /> },
      { path: 'wallbox', element: <Wallbox /> },
      { path: 'inverter', element: <Inverter /> },
      { path: 'energy', element: <EnergyFlow /> },
      { path: 'heating', element: <Heating /> },
      { path: 'grafana', element: <Grafana /> },
      { path: 'goE', element: <GoE /> },
      { path: 'update', element: <UpdatePage /> }
    ],
  },
])

createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
)
