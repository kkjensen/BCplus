function parseL7dF(l7df) {
	var dIdx = l7df.indexOf("."), cIdx = l7df.indexOf(",");
	if (dIdx >= 0) {
		if (cIdx >= 0) {
			if (dIdx < cIdx) {
				l7df = l7df.replace(".","");
				l7df = l7df.replace(",",".");				
			} else {
				l7df = l7df.replace(",","");				
			}
		}
	} else if (cIdx >= 0) {
		l7df = l7df.replace(",",".");
	}
	return parseFloat(l7df);
}
function cooDist(c1, c2) {
	var dx = c1[0] - c2[0], dy = c1[1] - c2[1], dz = c1[2] - c2[2];
	res = Math.sqrt(dx * dx + dy * dy + dz * dz);
	return res;
}
function dstCoo(dst) {
	var coos = dst.getElementsByTagName("td")[2].title.split("/");
	var res = [parseL7dF(coos[0]), parseL7dF(coos[1]), parseL7dF(coos[2])];
	return res;
}
function setTravel(dst, hop, sum) {
	var cells = dst.getElementsByTagName("td");
	cells[5].innerHTML = hop; /*.toLocaleString(???);*/
	cells[6].innerHTML = sum; /*.toLocaleString(???);*/
}
function compTravel() {
	var dstLs = document.querySelectorAll("#dests tbody tr");
	if (dstLs.length == 0) {
		return;
	}
	var i, sum = 0, oDst = dstLs[0], oCoo = dstCoo(oDst);
	setTravel(oDst, "–", "–");
	for (i = 1; i < dstLs.length; i++) {
		var nDst = dstLs[i], nCoo = dstCoo(nDst);
		var hop = cooDist(oCoo, nCoo);
		sum += hop;
		setTravel(nDst, hop.toFixed(2), sum.toFixed(2));
		oDst = nDst; oCoo = nCoo;
	}
}
function selShip(shId) {
	var rq = new Object();
	rq.topic = "travel";
	rq.op = "planShip";
	rq.shipId = parseInt(shId);
	var msg = JSON.stringify(rq);
	wsock.send(msg);
}
function tglHomeId(hId) {
	var rq = new Object();
	rq.topic = "travel";
	rq.op = "tglHomeId";
	rq.id = hId;
	var msg = JSON.stringify(rq);
	wsock.send(msg);	
}
function tglDstForm() {
	var hdr = document.getElementById("dsthdr");
	var frm = document.getElementById("addest");
	hdr.classList.toggle("hide");
	frm.classList.toggle("hide");
}
function editDst(btn) {
	var edNm = document.getElementById("destnm");
	var edCoos = document.getElementById("destcoo");
	var edNts = document.getElementById("destnts");
	var edTags = document.getElementById("desttags");
	var dst = btn.parentElement.parentElement.getElementsByTagName("td");
	showStatus(dst[0].innerText);
	edNm.value = dst[0].innerText;
	edCoos.value = dst[5].innerText;
	edNts.value = dst[6].innerText;
	edTags.value = dst[7].innerText;
}
function addDst() {
	var rq = new Object();
	rq.topic = "travel";
	rq.op = "addDst";
	rq.nm = document.getElementById("destnm").value;
	rq.coo = document.getElementById("destcoo").value;
	rq.note = document.getElementById("destnts").value;
	rq.tags = document.getElementById("desttags").value;
	var msg = JSON.stringify(rq);
	wsock.send(msg);		
}
function delDst(dId) {
	var rq = new Object();
	rq.topic = "travel";
	rq.op = "delDst";
	rq.id = dId;
	var msg = JSON.stringify(rq);
	wsock.send(msg);	
}
compTravel();
$( "#dests tbody" ).sortable({
	update: function(e, ui) {
		var dstls = document.querySelector("#dests tbody");
		var dsts = dstls.getElementsByTagName("tr");
		var i;
		var idls = new Array();
		for (i = 0; i < dsts.length; i++) {
			id = dsts[i].id.substring(3);
			idls.push(parseInt(id));
			dsts[i].id = "dst"+i;
		}
		compTravel();
		var rq = new Object();
		rq.topic = "travel";
		rq.op = "sortDst";
		rq.idls = idls;
		var msg = JSON.stringify(rq);
		wsock.send(msg);			
	}
});
