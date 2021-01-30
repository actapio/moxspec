package cpu

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/actapio/moxspec/loglet"
	"github.com/actapio/moxspec/util"
)

var log *loglet.Logger

func init() {
	log = loglet.NewLogger("cpu")
}

func newTopology() *Topology {
	t := new(Topology)
	t.path = "/sys/devices/system/cpu"
	t.packages = make(map[uint16]*Package)
	return t
}

func newPackage(pid uint16) *Package {
	p := new(Package)
	p.ID = pid
	p.nodes = make(map[uint16]*Node)
	return p
}

func newNode(nid uint16) *Node {
	n := new(Node)
	n.ID = nid
	n.cores = make(map[uint16]*Core)
	return n
}

func newCore(id uint16) *Core {
	c := new(Core)
	c.ID = id
	return c
}

// NewDecoder creates and initializes a Topology as Decoder
func NewDecoder() *Topology {
	return newTopology()
}

// Decode makes Topology satisfy the mox.Decoder interface
func (t *Topology) Decode() error {
	for _, cpudir := range util.FilterPrefixedDirs(t.path, "cpu") {

		topodir := filepath.Join(cpudir, "topology")
		if !util.Exists(topodir) {
			// e.g: cpufreq, cpuidle
			continue
		}

		log.Debugf("scanning %s", cpudir)

		pid, err := util.LoadUint16(filepath.Join(topodir, "physical_package_id"))
		if err != nil {
			log.Warnf("%s has no physical_package_id file", topodir)
			continue
		}

		nid, err := findNodeID(cpudir)
		if err != nil {
			log.Warn(err.Error())
			continue
		}

		cid, err := util.LoadUint16(filepath.Join(topodir, "core_id"))
		if err != nil {
			log.Warnf("%s has no core_id file", topodir)
			continue
		}

		log.Debugf("package = %d, node = %d, core = %d", pid, nid, cid)

		if !t.hasPackage(pid) {
			t.packages[pid] = newPackage(pid)
		}

		p := t.packages[pid]
		if !p.hasNode(nid) {
			p.nodes[nid] = newNode(nid)
		}

		pThrottle, err := util.LoadUint16(filepath.Join(cpudir, "thermal_throttle", "package_throttle_count"))
		if err == nil {
			p.ThrottleCount = pThrottle
		}

		n := p.nodes[nid]
		if !n.hasCore(cid) {
			c := newCore(cid)
			list := filepath.Join(topodir, "thread_siblings_list")
			ls, err := util.LoadString(list)
			if err != nil {
				log.Warnf("could not load %s", list)
				continue
			}
			c.Threads = parseListString(ls)
			cThrottle, err := util.LoadUint16(filepath.Join(cpudir, "thermal_throttle", "core_throttle_count"))
			if err == nil {
				c.ThrottleCount = cThrottle
			}

			n.cores[cid] = c
		}
	}
	return nil
}

func findNodeID(cpudir string) (uint16, error) {
	fs := util.FilterPrefixedLinks(cpudir, "node")
	if len(fs) != 1 {
		return 0, fmt.Errorf("valid node dir did not found in %s %v", cpudir, fs)
	}

	linkPath, err := os.Readlink(fs[0])
	if err != nil {
		return 0, err
	}

	idstr := strings.Replace(filepath.Base(linkPath), "node", "", 1)
	id, err := strconv.Atoi(idstr)

	return uint16(id), err
}
