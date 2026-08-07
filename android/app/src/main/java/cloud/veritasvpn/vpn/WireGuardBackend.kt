package cloud.veritasvpn.vpn

object WireGuardBackend {
    private var loaded = false

    init {
        try {
            System.loadLibrary("wg-go")
            loaded = true
        } catch (_: UnsatisfiedLinkError) {
            loaded = false
        }
    }

    fun isAvailable(): Boolean = loaded

    external fun wgTurnOn(ifname: String, tunFd: Int, settings: String): Int
    external fun wgTurnOff(ifname: String)
    external fun wgGetSocketV4(ifname: String): Int
    external fun wgGetSocketV6(ifname: String): Int
    external fun wgVersion(): String
}
