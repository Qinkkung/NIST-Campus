import { useEffect, useState } from 'react'
import { Wallet, ShieldCheck, Coins, BookOpen, Clock, ChevronRight } from 'lucide-react'

function App() {
  const [walletAddress, setWalletAddress] = useState('')
  const [trustScore, setTrustScore] = useState(82)
  const [orientationClaimed, setOrientationClaimed] = useState(false)
  const [claimingOrientation, setClaimingOrientation] = useState(false)
  const [claimMessage, setClaimMessage] = useState('')

  const connectWallet = () => {
    setWalletAddress('0xEC08...58ED')
  }

  useEffect(() => {
    if (!walletAddress) return
    fetch('http://localhost:8080/api/user?wallet=' + encodeURIComponent(walletAddress))
      .then(res => res.json())
      .then(data => {
        setTrustScore(data.trust_score)
        setOrientationClaimed(Boolean(data.orientation_claimed))
      })
      .catch(() => {})
  }, [walletAddress])

  const claimOrientation = async () => {
    if (!walletAddress || orientationClaimed || claimingOrientation) return
    setClaimingOrientation(true)
    setClaimMessage('')
    try {
      const res = await fetch('http://localhost:8080/api/orientation-claim', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ wallet_address: walletAddress }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'ไม่สามารถรับคะแนนได้')
      setTrustScore(data.new_score)
      setOrientationClaimed(true)
      setClaimMessage('รับคะแนนปฐมนิเทศ +5 สำเร็จ')
    } catch (error) {
      setClaimMessage(error.message)
    } finally {
      setClaimingOrientation(false)
    }
  }

  return (
    // พื้นหลังไล่ระดับสีแบบล้ำๆ (Mesh Gradient style)
    <div className="min-h-screen bg-[radial-gradient(ellipse_at_top_right,_var(--tw-gradient-stops))] from-blue-100 via-white to-indigo-50 p-4 md:p-8 font-sans text-slate-800">
      <div className="max-w-3xl mx-auto space-y-8">
        
        {/* Navbar */}
        <nav className="flex justify-between items-center py-4">
          <div className="flex items-center gap-2">
            <div className="bg-gradient-to-br from-blue-600 to-indigo-600 p-2 rounded-xl shadow-lg shadow-blue-200">
              <BookOpen className="text-white" size={24} />
            </div>
            <span className="text-2xl font-extrabold bg-clip-text text-transparent bg-gradient-to-r from-blue-700 to-indigo-700 tracking-tight">
              CampusPass
            </span>
          </div>
          
          <button 
            onClick={connectWallet}
            className="group flex items-center gap-2 px-5 py-2.5 bg-white/80 hover:bg-white backdrop-blur-md border border-slate-200/60 text-slate-700 font-semibold rounded-2xl transition-all shadow-sm hover:shadow-md"
          >
            <Wallet size={18} className="text-indigo-600 group-hover:scale-110 transition-transform" />
            {walletAddress ? walletAddress : 'Connect Wallet'}
          </button>
        </nav>

        {/* Dashboard Card (Glassmorphism) */}
        <section className="relative overflow-hidden bg-white/60 backdrop-blur-xl rounded-3xl p-8 border border-white/80 shadow-[0_8px_30px_rgb(0,0,0,0.04)]">
          {/* วงกลมตกแต่งแบคกราวน์ในการ์ด */}
          <div className="absolute -right-20 -top-20 w-64 h-64 bg-blue-400/10 rounded-full blur-3xl pointer-events-none"></div>
          
          <h1 className="text-2xl font-bold text-slate-800 mb-8">
            Welcome back, Student ✌️
          </h1>
          
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Balance Widget */}
            <div className="bg-white/80 p-6 rounded-2xl border border-slate-100 shadow-sm flex items-start gap-4">
              <div className="bg-blue-100 p-3 rounded-xl text-blue-600">
                <Coins size={24} />
              </div>
              <div>
                <p className="text-sm font-semibold text-slate-400 mb-1">NIST Balance</p>
                <div className="flex items-baseline gap-2">
                  <span className="text-4xl font-black text-slate-800 tracking-tight">200</span>
                  <span className="text-lg font-bold text-blue-600">NIST</span>
                </div>
              </div>
            </div>
            
            {/* Trust Score Widget */}
            <div className="bg-white/80 p-6 rounded-2xl border border-slate-100 shadow-sm flex items-start gap-4">
              <div className="bg-emerald-100 p-3 rounded-xl text-emerald-600">
                <ShieldCheck size={24} />
              </div>
              <div>
                <p className="text-sm font-semibold text-slate-400 mb-1">Trust Score</p>
                <div className="flex items-baseline gap-2">
                  <span className="text-4xl font-black text-slate-800 tracking-tight">{trustScore}</span>
                  <span className="text-lg font-medium text-slate-400">/ 100</span>
                </div>
                <div className="w-full bg-slate-100 h-2 rounded-full mt-3 overflow-hidden">
                  <div className="bg-gradient-to-r from-emerald-400 to-emerald-500 h-full rounded-full" style={{ width: `${trustScore}%` }}></div>
                </div>
              </div>
            </div>
          </div>

          <div className="mt-6 flex flex-col sm:flex-row sm:items-center gap-3">
            <button
              onClick={claimOrientation}
              disabled={!walletAddress || orientationClaimed || claimingOrientation}
              className="px-5 py-3 bg-indigo-600 hover:bg-indigo-700 disabled:bg-slate-300 disabled:cursor-not-allowed text-white font-semibold rounded-xl transition-colors"
            >
              {orientationClaimed ? '✓ รับคะแนนปฐมนิเทศแล้ว' : claimingOrientation ? 'กำลังรับคะแนน...' : 'รับคะแนนปฐมนิเทศ +5'}
            </button>
            {claimMessage && <span className="text-sm text-slate-600">{claimMessage}</span>}
          </div>
        </section>

        {/* Resources Section */}
        <section className="pt-4">
          <div className="flex items-center justify-between mb-6 px-2">
            <h2 className="text-xl font-bold text-slate-800">Available Resources</h2>
            <button className="text-sm font-semibold text-indigo-600 hover:text-indigo-700 flex items-center gap-1">
              View all <ChevronRight size={16} />
            </button>
          </div>
          
          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            {/* Room Card 1 */}
            <div className="group bg-white p-5 rounded-3xl border border-slate-100 shadow-sm hover:shadow-xl hover:shadow-indigo-100/50 transition-all duration-300">
              <div className="flex justify-between items-start mb-4">
                <div>
                  <h3 className="text-lg font-extrabold text-slate-800">A401 Study Room</h3>
                  <div className="flex items-center gap-1 text-sm text-slate-500 mt-1">
                    <Clock size={14} /> <span>Available Now</span>
                  </div>
                </div>
                <span className="bg-indigo-50 text-indigo-700 px-3 py-1 rounded-lg text-sm font-bold border border-indigo-100">
                  50 NIST
                </span>
              </div>
              <button className="w-full py-3 bg-slate-900 hover:bg-indigo-600 text-white font-semibold rounded-xl transition-colors duration-300 shadow-md">
                Reserve Resource
              </button>
            </div>

            {/* Room Card 2 */}
            <div className="group bg-white p-5 rounded-3xl border border-slate-100 shadow-sm hover:shadow-xl hover:shadow-indigo-100/50 transition-all duration-300">
              <div className="flex justify-between items-start mb-4">
                <div>
                  <h3 className="text-lg font-extrabold text-slate-800">B203 Classroom</h3>
                  <div className="flex items-center gap-1 text-sm text-slate-500 mt-1">
                    <Clock size={14} /> <span>Available Now</span>
                  </div>
                </div>
                <span className="bg-indigo-50 text-indigo-700 px-3 py-1 rounded-lg text-sm font-bold border border-indigo-100">
                  30 NIST
                </span>
              </div>
              <button className="w-full py-3 bg-slate-900 hover:bg-indigo-600 text-white font-semibold rounded-xl transition-colors duration-300 shadow-md">
                Reserve Resource
              </button>
            </div>
          </div>
        </section>

      </div>
    </div>
  )
}

export default App