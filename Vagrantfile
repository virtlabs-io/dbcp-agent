Vagrant.configure("2") do |config|
    # Box base
    config.vm.box = "ubuntu/focal64"
  
    # Parâmetros comuns de provisionamento para todos os nodes
    common_provision = <<-SHELL
      echo "Instalando Go 1.22..."
      wget -q https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
      sudo rm -rf /usr/local/go
      sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
      echo 'export PATH=$PATH:/usr/local/go/bin' >> /home/vagrant/.profile
      export PATH=$PATH:/usr/local/go/bin
      go version
  
      echo "Instalando dependências extras..."
      sudo apt-get update
      sudo apt-get install -y make git
  
      echo "Compilando dbcp-agent..."
      cd /home/vagrant/dbcp-agent
      make build || echo "Erro na compilação, verifique manualmente."
    SHELL
  
    # Node 1
    config.vm.define "db-node-1" do |node|
      node.vm.hostname = "db-node-1"
      node.vm.network "private_network", ip: "192.168.56.101"
      node.vm.synced_folder ".", "/home/vagrant/dbcp-agent", type: "virtualbox"
      node.vm.provision "shell", inline: common_provision
    end
  
    # Node 2
    config.vm.define "db-node-2" do |node|
      node.vm.hostname = "db-node-2"
      node.vm.network "private_network", ip: "192.168.56.102"
      node.vm.synced_folder ".", "/home/vagrant/dbcp-agent", type: "virtualbox"
      node.vm.provision "shell", inline: common_provision
    end
  
    # Node 3
    config.vm.define "db-node-3" do |node|
      node.vm.hostname = "db-node-3"
      node.vm.network "private_network", ip: "192.168.56.103"
      node.vm.synced_folder ".", "/home/vagrant/dbcp-agent", type: "virtualbox"
      node.vm.provision "shell", inline: common_provision
    end
  end
  